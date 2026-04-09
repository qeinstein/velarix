package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestBackupRestore(t *testing.T) {
	server, _ := setupTestServer(t)

	// 1. Assert a fact
	fact := map[string]interface{}{"id": "F1", "is_root": true, "manual_status": 1.0}
	body, _ := json.Marshal(fact)
	resp := performRequest(t, server, http.MethodPost, "/v1/s/sess1/facts", body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to create fact before backup: %d", resp.Code)
	}

	// 2. Perform Backup
	resp = performRequest(t, server, http.MethodGet, "/v1/org/backup", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Backup failed: status %d body %s", resp.Code, resp.Body.String())
	}

	var backupData bytes.Buffer
	backupData.ReadFrom(resp.Body)

	// 3. Clear Database by creating a new server (simulating disaster)
	server2, _ := setupTestServer(t)

	// 4. Perform Restore
	resp = performRequest(t, server2, http.MethodPost, "/v1/org/restore", backupData.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("Restore failed: status %d body %s", resp.Code, resp.Body.String())
	}

	// 5. Verify data exists after restore
	resp = performRequest(t, server2, http.MethodGet, "/v1/s/sess1/facts/F1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to retrieve fact after restore: status %d body %s", resp.Code, resp.Body.String())
	}

	var restoredFact struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&restoredFact)
	if restoredFact.ID != "F1" {
		t.Fatalf("Expected fact F1 after restore, got %s", restoredFact.ID)
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
		"password": "password123",
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
		"password": "password123",
	})
	resp := performAuthenticatedRequest(t, server, http.MethodPost, "/v1/auth/register", "", registerBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected registration to succeed, got %d body=%s", resp.Code, resp.Body.String())
	}

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "cookie-auth@example.com",
		"password": "password123",
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
		"new_password": "newpassword123",
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
		"password": "password123",
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
