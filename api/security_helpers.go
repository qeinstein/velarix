package api

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
)

const (
	minSecretBytes     = 32
	maxRequestBodySize = 4 * 1024 * 1024
)

func authCookiesEnabled() bool {
	return !envBool("VELARIX_DISABLE_AUTH_COOKIES")
}

func configuredSecretBytes(name string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, fmt.Errorf("%s is required and not set", name)
	}
	if decoded, ok := tryDecodeSecret(raw); ok {
		if len(decoded) < minSecretBytes {
			return nil, fmt.Errorf("%s must be at least %d bytes", name, minSecretBytes)
		}
		return decoded, nil
	}
	if len([]byte(raw)) < minSecretBytes {
		return nil, fmt.Errorf("%s must be at least %d bytes", name, minSecretBytes)
	}
	return []byte(raw), nil
}

func tryDecodeSecret(raw string) ([]byte, bool) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := enc.DecodeString(raw)
		if err == nil && len(decoded) > 0 {
			return decoded, true
		}
	}
	return nil, false
}

func mustConfiguredSecretBytes(name string) []byte {
	secret, err := configuredSecretBytes(name)
	if err == nil {
		return secret
	}
	slog.Error(name + " is required and not set — server will not start", "error", err)
	os.Exit(1)
	return nil
}

func ValidateRuntimeConfigOrExit() {
	_ = mustConfiguredSecretBytes("VELARIX_JWT_SECRET")
	_ = mustConfiguredSecretBytes("VELARIX_DECISION_TOKEN_SECRET")
	if authCookiesEnabled() && strings.TrimSpace(os.Getenv("VELARIX_ALLOWED_ORIGINS")) == "" {
		slog.Error("VELARIX_ALLOWED_ORIGINS is required when auth cookies are enabled — server will not start")
		os.Exit(1)
	}
}

func metricsRequestAllowed(r *http.Request) bool {
	if r == nil {
		return false
	}
	ip := net.ParseIP(clientIP(r))
	if ip != nil && ip.IsLoopback() {
		return true
	}
	allowedCIDRs := strings.TrimSpace(os.Getenv("VELARIX_METRICS_ALLOWED_CIDR"))
	if allowedCIDRs == "" || ip == nil {
		return false
	}
	for _, raw := range strings.Split(allowedCIDRs, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err != nil {
			slog.Warn("Ignoring invalid metrics CIDR", "cidr", raw)
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
