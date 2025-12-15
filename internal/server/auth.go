package server

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuthMiddleware enforces Basic Authentication if credentials are configured.
func (h *Handler) BasicAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If auth is not configured, skip check
		if h.cfg.App.Auth.User == "" || h.cfg.App.Auth.Pass == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || !h.checkCredentials(user, pass) {
			w.Header().Set("WWW-Authenticate", `Basic realm="MetricZ Exporter"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// checkCredentials uses constant time comparison to prevent timing attacks.
func (h *Handler) checkCredentials(user, pass string) bool {
	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(h.cfg.App.Auth.User)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(h.cfg.App.Auth.Pass)) == 1

	return userMatch && passMatch
}
