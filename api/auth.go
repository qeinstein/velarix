package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
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

	// Format: version$memory$iterations$parallelism$salt$hash
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

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hashed, err := hashPassword(body.Password)
	if err != nil {
		http.Error(w, "hashing failure", http.StatusInternalServerError)
		return
	}

	user := &store.User{
		Email:          body.Email,
		HashedPassword: hashed,
		OrgID:          "default",
		Keys:           []store.APIKey{},
	}

	if err := s.Store.SaveUser(user); err != nil {
		http.Error(w, "failed to save user", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "user created"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
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

func (s *Server) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, err := s.Store.GetUser(body.Email)
	if err != nil {
		// Do not leak if user exists, just return 200
		writeJSON(w, http.StatusOK, map[string]string{"status": "if email exists, a reset token has been generated"})
		return
	}

	// Generate 6-char token
	b := make([]byte, 3)
	rand.Read(b)
	token := fmt.Sprintf("%x", b)

	user.ResetToken = token
	user.ResetExpiry = time.Now().Add(15 * time.Minute).UnixMilli()
	s.Store.SaveUser(user)

	log.Printf("\n[AUTH] Password reset requested for %s\n[AUTH] TOKEN: %s (Expires in 15m)\n", user.Email, token)

	writeJSON(w, http.StatusOK, map[string]string{"status": "if email exists, a reset token has been generated"})
}

func (s *Server) handleResetConfirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
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
	user.ResetToken = "" // Clear token
	s.Store.SaveUser(user)

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}
