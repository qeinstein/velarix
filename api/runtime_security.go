package api

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const authCookieName = "velarix_token"

var authCookieNames = []string{authCookieName, "token"}

func runtimeEnv() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_ENV")))
	if env == "" {
		return "prod"
	}
	return env
}

func isDevLikeEnv() bool {
	switch runtimeEnv() {
	case "dev", "test":
		return true
	default:
		return false
	}
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func bootstrapAdminEnabled() bool {
	return envBool("VELARIX_ENABLE_BOOTSTRAP_ADMIN_KEY") || isDevLikeEnv()
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func currentUserTokenVersion(user interface{ GetTokenVersion() int64 }) int64 {
	if user == nil {
		return 1
	}
	if version := user.GetTokenVersion(); version > 0 {
		return version
	}
	return 1
}

func authCookieSameSite() http.SameSite {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("VELARIX_AUTH_COOKIE_SAMESITE"))) {
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteStrictMode
	}
}

func authCookieDomain() string {
	return strings.TrimSpace(os.Getenv("VELARIX_AUTH_COOKIE_DOMAIN"))
}

func authCookieSecure() bool {
	return !isDevLikeEnv()
}

func setAuthCookie(w http.ResponseWriter, token string, expiration time.Time) {
	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Expires:  expiration,
		MaxAge:   max(0, int(time.Until(expiration).Seconds())),
		Path:     "/",
		Domain:   authCookieDomain(),
		HttpOnly: true,
		SameSite: authCookieSameSite(),
		Secure:   authCookieSecure(),
	}
	http.SetCookie(w, cookie)
}

func clearAuthCookies(w http.ResponseWriter) {
	for _, name := range authCookieNames {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   authCookieDomain(),
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			SameSite: authCookieSameSite(),
			Secure:   authCookieSecure(),
		})
	}
}

func authTokenFromRequest(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	for _, name := range authCookieNames {
		cookie, err := r.Cookie(name)
		if err != nil {
			continue
		}
		if token := strings.TrimSpace(cookie.Value); token != "" {
			return token
		}
	}
	return ""
}

func addVaryHeader(h http.Header, key string) {
	if h == nil || key == "" {
		return
	}
	current := h.Values("Vary")
	for _, existing := range current {
		for _, part := range strings.Split(existing, ",") {
			if strings.EqualFold(strings.TrimSpace(part), key) {
				return
			}
		}
	}
	h.Add("Vary", key)
}

func originAllowed(origin, allowed string) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	for _, part := range strings.Split(allowed, ",") {
		if strings.TrimSpace(part) == origin {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if envBool("VELARIX_TRUST_PROXY_HEADERS") {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			if idx := strings.Index(forwarded, ","); idx >= 0 {
				forwarded = forwarded[:idx]
			}
			if forwarded = strings.TrimSpace(forwarded); forwarded != "" {
				return forwarded
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-Ip")); realIP != "" {
			return realIP
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func authRouteRateLimit(route string) (ipLimit int, ipWindow time.Duration, emailLimit int, emailWindow time.Duration) {
	switch route {
	case "register":
		return 10, 10 * time.Minute, 3, 30 * time.Minute
	case "reset_request":
		return 6, 15 * time.Minute, 6, 15 * time.Minute
	case "reset_confirm":
		return 10, 15 * time.Minute, 6, 15 * time.Minute
	default:
		return 20, time.Minute, 8, 15 * time.Minute
	}
}

func setRateLimitResponseHeaders(w http.ResponseWriter, limit int, window, retryAfter time.Duration) {
	if w == nil {
		return
	}
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Round(time.Second).Seconds())))
	if limit > 0 {
		w.Header().Set("X-Velarix-RateLimit-Limit", strconv.Itoa(limit))
	}
	if window > 0 {
		w.Header().Set("X-Velarix-RateLimit-Window", strconv.Itoa(int(window.Seconds())))
	}
}

func (s *Server) enforceAuthRateLimit(w http.ResponseWriter, r *http.Request, route string, email string) bool {
	ipLimit, ipWindow, emailLimit, emailWindow := authRouteRateLimit(route)
	if ip := clientIP(r); ip != "" && ipLimit > 0 && ipWindow > 0 {
		key := fmt.Sprintf("auth:%s:ip:%s", route, ip)
		if allowed, retryAfter := s.checkRateLimit(key, ipLimit, ipWindow); !allowed {
			setRateLimitResponseHeaders(w, ipLimit, ipWindow, retryAfter)
			http.Error(w, "auth rate limit exceeded", http.StatusTooManyRequests)
			return false
		}
	}
	email = normalizeEmail(email)
	if email != "" && emailLimit > 0 && emailWindow > 0 {
		key := fmt.Sprintf("auth:%s:email:%s", route, email)
		if allowed, retryAfter := s.checkRateLimit(key, emailLimit, emailWindow); !allowed {
			setRateLimitResponseHeaders(w, emailLimit, emailWindow, retryAfter)
			http.Error(w, "auth rate limit exceeded", http.StatusTooManyRequests)
			return false
		}
	}
	return true
}

func idempotencyReplayWindow() time.Duration {
	raw := strings.TrimSpace(os.Getenv("VELARIX_IDEMPOTENCY_TTL_HOURS"))
	if raw == "" {
		return 24 * time.Hour
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 || hours > 24*30 {
		return 24 * time.Hour
	}
	return time.Duration(hours) * time.Hour
}

func capturedIdempotencyHeaders(h http.Header) map[string]string {
	if h == nil {
		return nil
	}
	allowed := []string{
		"Location",
		"ETag",
		"Retry-After",
		"X-Trace-Id",
		"X-Idempotency-Key",
		"X-Velarix-Backpressure",
		"X-Velarix-RateLimit-Limit",
		"X-Velarix-RateLimit-Window",
	}
	out := map[string]string{}
	for _, key := range allowed {
		if value := strings.TrimSpace(h.Get(key)); value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func persistIdempotencyResponse(status int, contentType string, bodyLen int) bool {
	if bodyLen == 0 {
		return false
	}
	if status < 200 || status >= 500 {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "text/plain")
}
