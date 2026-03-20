package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"velarix/store"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

var jwtKey = []byte("velarix_secret_console_key") // In production, load from ENV

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// Argon2 Parameters
const (
	memory      = 64 * 1024
	iterations  = 3
	parallelism = 2
	saltLength  = 16
	keyLength   = 32
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, iterations, parallelism, b64Salt, b64Hash)
	return encodedHash, nil
}

func comparePassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid combined hash format")
	}

	var m, t, p uint32
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	comparisonHash := argon2.IDKey([]byte(password), salt, t, m, uint8(p), uint32(len(hash)))

	return subtle.ConstantTimeCompare(hash, comparisonHash) == 1, nil
}

type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"securepassword123"`
}

type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"securepassword123"`
}

type ResetRequest struct {
	Email string `json:"email" example:"user@example.com"`
}

type ResetConfirmRequest struct {
	Email       string `json:"email" example:"user@example.com"`
	Token       string `json:"token" example:"a1b2c3"`
	NewPassword string `json:"new_password" example:"newsecurepassword456"`
}

type GenerateKeyRequest struct {
	Email string `json:"email" example:"user@example.com"`
	Label string `json:"label" example:"Production"`
}

func getUserEmail(r *http.Request) string {
	val := r.Context().Value(userEmailKey)
	if val == nil {
		return ""
	}
	return val.(string)
}

func getUserRole(r *http.Request) string {
	val := r.Context().Value(userRoleKey)
	if val == nil {
		return ""
	}
	return val.(string)
}

// handleRegister godoc
// @Summary Register a new user
// @Description Create a new user account with Argon2id password hashing. No authentication required.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration details"
// @Success 201 {object} map[string]string "user created"
// @Failure 400 {string} string "invalid request"
// @Failure 500 {string} string "internal error"
// @Router /auth/register [post]
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hashed, err := hashPassword(body.Password)
	if err != nil {
		http.Error(w, "hashing failure", http.StatusInternalServerError)
		return
	}

	role := "member"
	adminEmail := os.Getenv("VELARIX_ADMIN_EMAIL")
	if adminEmail != "" && body.Email == adminEmail {
		role = "admin"
	}

	b := make([]byte, 8)
	rand.Read(b)
	orgID := "org_" + hex.EncodeToString(b)

	user := &store.User{
		Email:          body.Email,
		HashedPassword: hashed,
		OrgID:          orgID,
		Role:           role,
		Keys:           []store.APIKey{},
	}

	org := &store.Organization{
		ID:          orgID,
		Name:        body.Email + " Organization",
		CreatedAt:   time.Now().UnixMilli(),
		IsSuspended: false,
	}

	if err := s.Store.SaveOrganization(org); err != nil {
		http.Error(w, "failed to save organization", http.StatusInternalServerError)
		return
	}

	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	slog.Info("User registered", "email", body.Email, "org_id", orgID, "role", role)

	writeJSON(w, http.StatusCreated, map[string]string{"status": "user created"})
}

// handleLogin godoc
// @Summary Login user
// @Description Authenticate user and return a JWT for console access. No authentication required.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} map[string]string "token"
// @Failure 401 {string} string "invalid credentials"
// @Router /auth/login [post]
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.Store.GetUser(body.Email)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	match, err := comparePassword(body.Password, user.HashedPassword)
	if err != nil || !match {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: body.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, "token generation failure", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		Expires: expirationTime,
	})

	writeJSON(w, http.StatusOK, map[string]string{"token": tokenString})
}

// handleResetRequest godoc
// @Summary Request password reset
// @Description Generate a reset token and log it to the server console. No authentication required.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ResetRequest true "User email"
// @Success 200 {object} map[string]string "status"
// @Router /auth/reset-request [post]
func (s *Server) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	var body ResetRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.Store.GetUser(body.Email)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "if email exists, a reset token has been generated"})
		return
	}

	b := make([]byte, 3)
	rand.Read(b)
	token := fmt.Sprintf("%x", b)

	user.ResetToken = token
	user.ResetExpiry = time.Now().Add(15 * time.Minute).UnixMilli()
	s.Store.SaveUser(user)

	slog.Info("Password reset requested", "email", user.Email, "token", token, "expires_in", "15m")

	writeJSON(w, http.StatusOK, map[string]string{"status": "if email exists, a reset token has been generated"})
}

// handleResetConfirm godoc
// @Summary Confirm password reset
// @Description Update password using a reset token provided via console logs. No authentication required.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ResetConfirmRequest true "Reset details"
// @Success 200 {object} map[string]string "status"
// @Failure 401 {string} string "invalid token"
// @Router /auth/reset-confirm [post]
func (s *Server) handleResetConfirm(w http.ResponseWriter, r *http.Request) {
	var body ResetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.Store.GetUser(body.Email)
	if err != nil {
		http.Error(w, "invalid token or expired", http.StatusUnauthorized)
		return
	}

	if user.ResetToken == "" || user.ResetToken != body.Token || time.Now().UnixMilli() > user.ResetExpiry {
		http.Error(w, "invalid token or expired", http.StatusUnauthorized)
		return
	}

	hashed, _ := hashPassword(body.NewPassword)
	user.HashedPassword = hashed
	user.ResetToken = ""
	s.Store.SaveUser(user)

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

// handleListKeys godoc
// @Summary List API keys
// @Description Retrieve all API keys associated with the user.
// @Tags Auth
// @Security Bearer
// @Accept json
// @Produce json
// @Success 200 {array} store.APIKey
// @Failure 401 {string} string "unauthorized"
// @Failure 404 {string} string "user not found"
// @Router /keys [get]
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	email := getUserEmail(r)
	if email == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, user.Keys)
}

// handleRevokeKey godoc
// @Summary Revoke an API key
// @Description Mark an API key as revoked. Revoked keys return 401.
// @Tags Auth
// @Security Bearer
// @Accept json
// @Produce json
// @Param key path string true "API Key" example("vx_12345")
// @Success 200 {object} map[string]string "status"
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "key not found"
// @Router /keys/{key} [delete]
func (s *Server) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	keyToRevoke := r.PathValue("key")
	email := getUserEmail(r)
	role := getUserRole(r)

	if email == "" || keyToRevoke == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "forbidden: admin role required to revoke keys", http.StatusForbidden)
		return
	}

	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	found := false
	for i, k := range user.Keys {
		if k.Key == keyToRevoke {
			user.Keys[i].IsRevoked = true
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to revoke key", http.StatusInternalServerError)
		return
	}

	s.auditAdmin("admin", email, "revoke_key", map[string]interface{}{"key": keyToRevoke})
	slog.Info("API Key revoked", "email", email, "key", keyToRevoke)

	writeJSON(w, http.StatusOK, map[string]string{"status": "key revoked"})
}

// handleGenerateKey godoc
// @Summary Generate a new API key
// @Description Create a new labeled API key for the user.
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body GenerateKeyRequest true "Key details"
// @Success 201 {object} store.APIKey
// @Failure 400 {string} string "invalid request"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "user not found"
// @Router /keys/generate [post]
func (s *Server) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	var body GenerateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	email := getUserEmail(r)
	role := getUserRole(r)

	if email == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "forbidden: admin role required to generate keys", http.StatusForbidden)
		return
	}

	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	newKey := "vx_" + hex.EncodeToString(b)

	apiKey := store.APIKey{
		Key:       newKey,
		Label:     body.Label,
		CreatedAt: time.Now().UnixMilli(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
		IsRevoked: false,
	}

	user.Keys = append(user.Keys, apiKey)

	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to save key", http.StatusInternalServerError)
		return
	}

	s.auditAdmin("admin", email, "generate_key", map[string]interface{}{"label": body.Label, "key": apiKey.Key})
	slog.Info("API Key generated", "email", email, "label", body.Label, "expires_at", apiKey.ExpiresAt)

	writeJSON(w, http.StatusCreated, apiKey)
}

// handleRotateKey godoc
// @Summary Rotate an API key
// @Description Revokes the old key and generates a new one with the same label.
// @Tags Auth
// @Security Bearer
// @Accept json
// @Produce json
// @Param key path string true "Old API Key" example("vx_12345")
// @Success 201 {object} store.APIKey
// @Failure 401 {string} string "unauthorized"
// @Failure 403 {string} string "forbidden"
// @Failure 404 {string} string "key not found"
// @Router /keys/{key}/rotate [post]
func (s *Server) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	keyToRotate := r.PathValue("key")
	email := getUserEmail(r)
	role := getUserRole(r)

	if email == "" || keyToRotate == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if role != "admin" {
		http.Error(w, "forbidden: admin role required to rotate keys", http.StatusForbidden)
		return
	}

	user, err := s.Store.GetUser(email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	var oldLabel string
	found := false
	for i, k := range user.Keys {
		if k.Key == keyToRotate {
			if k.IsRevoked {
				http.Error(w, "key already revoked", http.StatusBadRequest)
				return
			}
			user.Keys[i].IsRevoked = true
			oldLabel = k.Label
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	newKey := "vx_" + hex.EncodeToString(b)

	apiKey := store.APIKey{
		Key:       newKey,
		Label:     oldLabel,
		CreatedAt: time.Now().UnixMilli(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UnixMilli(),
		IsRevoked: false,
	}

	user.Keys = append(user.Keys, apiKey)

	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to rotate key", http.StatusInternalServerError)
		return
	}

	s.auditAdmin("admin", email, "rotate_key", map[string]interface{}{"old_key": keyToRotate, "new_key": newKey})
	slog.Info("API Key rotated", "email", email, "old_key", keyToRotate, "new_label", oldLabel)

	writeJSON(w, http.StatusCreated, apiKey)
}
