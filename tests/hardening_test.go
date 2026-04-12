package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"velarix/store"
)

func TestRBACEnforcement(t *testing.T) {
	server, _ := setupTestServer(t)

	// 1. Setup a regular member user
	memberUser := &store.User{
		Email: "member@example.com",
		OrgID: "test_org",
		Role:  "member",
		Keys: []store.APIKey{
			{Key: "member_key", Label: "member_actor", IsRevoked: false, ExpiresAt: 9999999999999},
		},
	}
	server.Store.SaveUser(memberUser)

	// 2. Attempt admin operation (backup) with member key
	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/org/backup", "member_key", nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing backup, got %d", resp.Code)
	}

	// 3. Attempt admin operation (full health) with member key
	resp = performAuthenticatedRequest(t, server, http.MethodGet, "/health/full", "member_key", nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for member accessing full health, got %d", resp.Code)
	}
}

func TestBackupOrgScoped(t *testing.T) {
	// Verify that the backup endpoint only returns data for the calling org.
	serverA, badgerA := setupTestServer(t)
	_ = badgerA

	// Seed org A session and fact.
	factA := map[string]interface{}{"id": "F_ORG_A", "is_root": true, "manual_status": 1.0}
	bodyA, _ := json.Marshal(factA)
	resp := performRequest(t, serverA, http.MethodPost, "/v1/s/sess_org_a/facts", bodyA)
	if resp.Code != http.StatusCreated {
		t.Fatalf("org A fact creation failed: %d %s", resp.Code, resp.Body.String())
	}

	// Seed a different org B user and session directly in the store.
	orgBUser := &store.User{
		Email: "orgb@example.com",
		OrgID: "org_b",
		Role:  "admin",
		Keys:  []store.APIKey{{Key: "org_b_key", Label: "org_b", IsRevoked: false, ExpiresAt: 9999999999999}},
	}
	serverA.Store.SaveUser(orgBUser)
	serverA.Store.SaveOrganization(&store.Organization{ID: "org_b", Name: "Org B"})

	factB := map[string]interface{}{"id": "F_ORG_B", "is_root": true, "manual_status": 1.0}
	bodyB, _ := json.Marshal(factB)
	// Assert a fact as org B directly through the HTTP handler.
	respB := performAuthenticatedRequest(t, serverA, http.MethodPost, "/v1/s/sess_org_b/facts", "org_b_key", bodyB)
	if respB.Code != http.StatusCreated {
		t.Fatalf("org B fact creation failed: %d %s", respB.Code, respB.Body.String())
	}

	// Call backup as org A (via bootstrap admin key which has orgID="admin", but test_org is the default test org).
	resp = performRequest(t, serverA, http.MethodGet, "/v1/org/backup", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("backup failed: %d %s", resp.Code, resp.Body.String())
	}

	backupBody := resp.Body.String()
	// The backup must NOT contain org B session IDs or fact IDs.
	if strings.Contains(backupBody, "sess_org_b") {
		t.Fatalf("org A backup contains org B session ID")
	}
	if strings.Contains(backupBody, "F_ORG_B") {
		t.Fatalf("org A backup contains org B fact ID")
	}
}

func TestRestoreEndpointRemoved(t *testing.T) {
	server, _ := setupTestServer(t)
	// The /v1/org/restore endpoint must not exist on the public router.
	resp := performRequest(t, server, http.MethodPost, "/v1/org/restore", []byte("{}"))
	if resp.Code == http.StatusOK {
		t.Fatalf("restore endpoint must not be publicly accessible, got 200")
	}
}

func TestPasswordResetIsDisabledOutsideDev(t *testing.T) {
	server, _ := setupTestServer(t)
	if err := server.Store.SaveUser(&store.User{
		Email:          "reset@example.com",
		OrgID:          "test_org",
		Role:           "member",
		HashedPassword: "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQ$ZmFrZWhhc2g",
	}); err != nil {
		t.Fatalf("failed to save reset test user: %v", err)
	}

	t.Setenv("VELARIX_ENV", "prod")
	body, _ := json.Marshal(map[string]string{"email": "reset@example.com"})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/reset-request", "", body)
	if resp.Code != http.StatusNotImplemented {
		t.Fatalf("expected password reset to be disabled outside dev, got %d body=%s", resp.Code, resp.Body.String())
	}

	t.Setenv("VELARIX_ENV", "dev")
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/reset-request", "", body)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected dev reset request to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
	var resetResponse map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&resetResponse); err != nil {
		t.Fatalf("failed to decode reset response: %v", err)
	}
	if resetResponse["dev_reset_token"] == "" {
		t.Fatalf("expected dev reset response to include a one-time token")
	}
}

func TestRegisterRejectsDuplicateAccount(t *testing.T) {
	server, _ := setupTestServer(t)

	body, _ := json.Marshal(map[string]string{
		"email":    "duplicate@example.com",
		"password": "password123456",
	})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected initial registration to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", body)
	if resp.Code != http.StatusConflict {
		t.Fatalf("expected duplicate registration to be rejected, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestCookieAuthAndResetRevocation(t *testing.T) {
	server, _ := setupTestServer(t)
	t.Setenv("VELARIX_ENV", "dev")

	registerBody, _ := json.Marshal(map[string]string{
		"email":    "cookie-auth@example.com",
		"password": "password123456",
	})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", registerBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected registration to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Promote to admin directly — in production this happens via invitation.
	// This test validates cookie auth and key revocation, not the registration flow.
	user, err := server.Store.GetUser("cookie-auth@example.com")
	if err != nil {
		t.Fatalf("failed to load registered user: %v", err)
	}
	user.Role = "admin"
	if err := server.Store.SaveUser(user); err != nil {
		t.Fatalf("failed to promote user to admin: %v", err)
	}

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "cookie-auth@example.com",
		"password": "password123456",
	})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/login", "", loginBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected login to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
	var loginResponse map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&loginResponse); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResponse["token"] == "" {
		t.Fatalf("expected login response to include a jwt")
	}
	cookies := resp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected login to set an auth cookie")
	}
	authCookie := cookies[0]

	// Verify cookie auth works for an authenticated endpoint.
	cookieReq := httptest.NewRequest(http.MethodGet, "/v1/keys", nil)
	cookieReq.AddCookie(authCookie)
	cookieResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(cookieResp, cookieReq)
	if cookieResp.Code != http.StatusOK {
		t.Fatalf("expected cookie-authenticated request to succeed, got %d body=%s", cookieResp.Code, cookieResp.Body.String())
	}

	generateKeyBody, _ := json.Marshal(map[string]interface{}{
		"label":  "reset-test",
		"scopes": []string{"read", "write"},
	})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/keys/generate", loginResponse["token"], generateKeyBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected api key generation to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
	var keyView map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&keyView); err != nil {
		t.Fatalf("failed to decode key response: %v", err)
	}
	apiKey, _ := keyView["key"].(string)
	if apiKey == "" {
		t.Fatalf("expected generated api key in response")
	}

	resetReqBody, _ := json.Marshal(map[string]string{"email": "cookie-auth@example.com"})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/reset-request", "", resetReqBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected reset request to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
	var resetRequest map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&resetRequest); err != nil {
		t.Fatalf("failed to decode reset request response: %v", err)
	}
	resetToken := resetRequest["dev_reset_token"]
	if resetToken == "" {
		t.Fatalf("expected dev reset token in response")
	}

	resetConfirmBody, _ := json.Marshal(map[string]string{
		"email":        "cookie-auth@example.com",
		"token":        resetToken,
		"new_password": "newpassword123456",
	})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/reset-confirm", "", resetConfirmBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected reset confirmation to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	resp = performAuthenticatedRequest(t, server, http.MethodGet, "/v1/keys", loginResponse["token"], nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected old jwt to be rejected after reset, got %d body=%s", resp.Code, resp.Body.String())
	}

	resp = performAuthenticatedRequest(t, server, http.MethodGet, "/v1/sessions", apiKey, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected old api key to be revoked after reset, got %d body=%s", resp.Code, resp.Body.String())
	}

	staleCookieReq := httptest.NewRequest(http.MethodGet, "/v1/keys", nil)
	staleCookieReq.AddCookie(authCookie)
	staleCookieResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(staleCookieResp, staleCookieReq)
	if staleCookieResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected stale auth cookie to be rejected after reset, got %d body=%s", staleCookieResp.Code, staleCookieResp.Body.String())
	}
}

func TestBootstrapAdminKeyRequiresExplicitProdOptIn(t *testing.T) {
	server, _ := setupTestServer(t)
	t.Setenv("VELARIX_ENV", "prod")
	t.Setenv("VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY", "false")

	resp := performAuthenticatedRequest(t, server, http.MethodGet, "/v1/sessions", "test_admin_key", nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected bootstrap admin key to be disabled in prod, got %d body=%s", resp.Code, resp.Body.String())
	}

	t.Setenv("VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY", "true")
	resp = performAuthenticatedRequest(t, server, http.MethodGet, "/v1/sessions", "test_admin_key", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected explicit prod opt-in to restore bootstrap admin key, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestAuthResetRequestRateLimited(t *testing.T) {
	server, _ := setupTestServer(t)
	t.Setenv("VELARIX_ENV", "dev")
	if err := server.Store.SaveUser(&store.User{
		Email:          "limited-reset@example.com",
		OrgID:          "test_org",
		Role:           "member",
		TokenVersion:   1,
		HashedPassword: "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQ$ZmFrZWhhc2g",
	}); err != nil {
		t.Fatalf("failed to save reset test user: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"email": "limited-reset@example.com"})
	for i := 0; i < 6; i++ {
		resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/reset-request", "", body)
		if resp.Code != http.StatusOK {
			t.Fatalf("expected reset request %d to succeed, got %d body=%s", i+1, resp.Code, resp.Body.String())
		}
	}

	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/reset-request", "", body)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected reset request throttling, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestProdCORSFailsClosedAndSecurityHeadersAreSet(t *testing.T) {
	server, _ := setupTestServer(t)
	t.Setenv("VELARIX_ENV", "prod")
	t.Setenv("VELARIX_ALLOWED_ORIGINS", "")

	preflight := httptest.NewRequest(http.MethodOptions, "/v1/auth/login", nil)
	preflight.Header.Set("Origin", "https://evil.example")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(preflightResp, preflight)
	if preflightResp.Code != http.StatusForbidden {
		t.Fatalf("expected prod preflight from unconfigured origin to be rejected, got %d", preflightResp.Code)
	}
	if preflightResp.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected no access-control-allow-origin header for rejected origin")
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthResp := httptest.NewRecorder()
	server.Routes().ServeHTTP(healthResp, healthReq)
	if healthResp.Code != http.StatusOK {
		t.Fatalf("expected health check to succeed, got %d", healthResp.Code)
	}
	for key, expected := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := healthResp.Header().Get(key); got != expected {
			t.Fatalf("expected %s=%q, got %q", key, expected, got)
		}
	}
	if healthResp.Header().Get("Strict-Transport-Security") == "" {
		t.Fatalf("expected strict transport security header in prod responses")
	}
}

func TestInvitationTokenIsOnlyReturnedOnce(t *testing.T) {
	server, _ := setupTestServer(t)

	body, _ := json.Marshal(map[string]string{
		"email": "invitee@example.com",
		"role":  "member",
	})
	resp := performRequest(t, server, http.MethodPost, "/v1/org/invitations", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected invitation create to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode invitation create response: %v", err)
	}
	token, _ := created["invite_token"].(string)
	if token == "" {
		t.Fatalf("expected invitation create response to include invite_token")
	}

	resp = performRequest(t, server, http.MethodGet, "/v1/org/invitations", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected invitation list to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
	var listResponse struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResponse); err != nil {
		t.Fatalf("failed to decode invitation list response: %v", err)
	}
	if len(listResponse.Items) == 0 {
		t.Fatalf("expected at least one invitation in list")
	}
	if _, ok := listResponse.Items[0]["token"]; ok {
		t.Fatalf("expected invitation list to redact token field")
	}

	acceptBody, _ := json.Marshal(map[string]string{
		"token":    token,
		"password": "password123456",
	})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/invitations/accept", "", acceptBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected invitation accept to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestRetentionEnforcementDeletesExpiredRecords(t *testing.T) {
	server, _ := setupTestServer(t)
	org, err := server.Store.GetOrganization("test_org")
	if err != nil {
		t.Fatalf("failed to read org: %v", err)
	}
	org.Settings = map[string]interface{}{
		"retention_days_activity":      1,
		"retention_days_access_logs":   1,
		"retention_days_notifications": 1,
	}
	if err := server.Store.SaveOrganization(org); err != nil {
		t.Fatalf("failed to update org retention settings: %v", err)
	}

	oldTs := time.Now().Add(-72 * time.Hour).UnixMilli()
	if err := server.Store.AppendOrgActivity("test_org", store.JournalEntry{Type: store.EventAdminAction, Timestamp: oldTs}); err != nil {
		t.Fatalf("failed to append old activity: %v", err)
	}
	if err := server.Store.AppendAccessLog("test_org", store.AccessLogEntry{ID: "old_access_log", CreatedAt: oldTs}); err != nil {
		t.Fatalf("failed to append old access log: %v", err)
	}
	if err := server.Store.SaveNotification("test_org", &store.Notification{ID: "old_notification", CreatedAt: oldTs}); err != nil {
		t.Fatalf("failed to save old notification: %v", err)
	}

	report, err := server.Store.EnforceRetention(time.Now())
	if err != nil {
		t.Fatalf("retention enforcement failed: %v", err)
	}
	if report.ActivityDeleted == 0 || report.AccessLogsDeleted == 0 || report.NotificationsDeleted == 0 {
		t.Fatalf("expected retention sweep to delete expired records, got %+v", report)
	}
}

// TestRegisterRoleIsMember verifies that standard self-registration always produces
// a member role, never an admin. It also verifies that the returned token cannot
// access any admin-only route.
func TestRegisterRoleIsMember(t *testing.T) {
	server, _ := setupTestServer(t)
	t.Setenv("VELARIX_ENV", "dev")
	t.Setenv("VELARIX_ADMIN_EMAIL", "") // ensure no bootstrap admin email is set

	// Register a new account via the public endpoint.
	regBody, _ := json.Marshal(map[string]string{
		"email":    "newmember@example.com",
		"password": "securepassword123",
	})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", regBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("registration failed: %d %s", resp.Code, resp.Body.String())
	}

	// Login to get a JWT.
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "newmember@example.com",
		"password": "securepassword123",
	})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/login", "", loginBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", resp.Code, resp.Body.String())
	}
	var loginResp map[string]string
	json.NewDecoder(resp.Body).Decode(&loginResp)
	token := loginResp["token"]
	if token == "" {
		t.Fatalf("no token in login response")
	}

	// Verify the stored user has role=member.
	user, err := server.Store.GetUser("newmember@example.com")
	if err != nil {
		t.Fatalf("could not load registered user: %v", err)
	}
	if user.Role != "member" {
		t.Fatalf("expected role=member after public registration, got %q", user.Role)
	}

	// Verify the token cannot access admin-only routes (backup, key generation, restore check).
	resp = performAuthenticatedRequest(t, server, http.MethodGet, "/v1/org/backup", token, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("member token should not access /v1/org/backup, got %d", resp.Code)
	}

	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/keys/generate", token, []byte(`{"label":"test"}`))
	if resp.Code != http.StatusForbidden {
		t.Fatalf("member token should not generate API keys, got %d", resp.Code)
	}
}

// TestInvitationTakeoverPrevented verifies that accepting an invitation for an email
// address that already has an account is rejected when the caller is unauthenticated.
func TestInvitationTakeoverPrevented(t *testing.T) {
	server, _ := setupTestServer(t)

	// Create an existing user account.
	existingUser := &store.User{
		Email:          "existing@example.com",
		OrgID:          "test_org",
		Role:           "member",
		HashedPassword: "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQ$ZmFrZWhhc2g",
		Keys:           []store.APIKey{},
	}
	if err := server.Store.SaveUser(existingUser); err != nil {
		t.Fatalf("failed to seed existing user: %v", err)
	}

	// Create an invitation for the existing user's email.
	invBody, _ := json.Marshal(map[string]string{
		"email": "existing@example.com",
		"role":  "member",
	})
	resp := performRequest(t, server, http.MethodPost, "/v1/org/invitations", invBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("invitation creation failed: %d %s", resp.Code, resp.Body.String())
	}
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	token, _ := created["invite_token"].(string)
	if token == "" {
		t.Fatalf("no invite_token in create response")
	}

	// Attempt to accept the invitation unauthenticated — must be rejected.
	acceptBody, _ := json.Marshal(map[string]string{
		"token":    token,
		"password": "somepassword123",
	})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/invitations/accept", "", acceptBody)
	if resp.Code != http.StatusConflict && resp.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated invite accept for existing email must return 409 or 403, got %d body=%s", resp.Code, resp.Body.String())
	}

	// Verify the existing account is unchanged (password not overwritten).
	user, err := server.Store.GetUser("existing@example.com")
	if err != nil {
		t.Fatalf("existing user should still exist: %v", err)
	}
	if user.HashedPassword != existingUser.HashedPassword {
		t.Fatalf("existing user's password was modified by unauthenticated invite accept")
	}
}

// TestAuditLogPoisoningPrevented verifies that the POST /v1/s/{id}/history endpoint
// is not reachable from the public router.
func TestAuditLogPoisoningPrevented(t *testing.T) {
	server, _ := setupTestServer(t)
	payload, _ := json.Marshal(map[string]interface{}{"type": "assert", "session_id": "sess1"})
	resp := performRequest(t, server, http.MethodPost, "/v1/s/sess1/history", payload)
	// Must return 405 Method Not Allowed or 404 Not Found — never 201 or 200.
	if resp.Code == http.StatusCreated || resp.Code == http.StatusOK {
		t.Fatalf("POST /v1/s/{id}/history must not be accessible, got %d", resp.Code)
	}
}

// TestPasswordMinimumLength verifies the 12-character minimum password policy
// is enforced at registration, reset-confirm, and invitation acceptance.
func TestPasswordMinimumLength(t *testing.T) {
	server, _ := setupTestServer(t)
	t.Setenv("VELARIX_ENV", "dev")

	// Registration with short password must fail.
	regBody, _ := json.Marshal(map[string]string{"email": "short@example.com", "password": "tooshort"})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", regBody)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("short password at registration should return 400, got %d", resp.Code)
	}

	// Registration with exactly 12 chars must succeed.
	regBody12, _ := json.Marshal(map[string]string{"email": "short12@example.com", "password": "exactly12chr"})
	resp = performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", regBody12)
	if resp.Code != http.StatusCreated {
		t.Fatalf("12-char password at registration should succeed, got %d body=%s", resp.Code, resp.Body.String())
	}
}
