package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// JSONTranslatorMiddleware checks for "format=json" query param.
// If present, it transparently decodes a JSON array of strings from the request body
// and feeds the underlying handler with a stream of newline-separated strings.
func (h *Handler) JSONTranslatorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") == "json" {
			r.Body = newJSONToTextReader(r.Body)
		}

		next.ServeHTTP(w, r)
	})
}

// newJSONToTextReader creates a reader that transforms a JSON array stream into text stream.
func newJSONToTextReader(originalBody io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		defer func() { _ = pw.Close() }()
		defer func() { _ = originalBody.Close() }()

		dec := json.NewDecoder(originalBody)
		if token, err := dec.Token(); err != nil || token != json.Delim('[') {
			_ = pw.CloseWithError(fmt.Errorf("expected JSON array start, got %v", token))
			return
		}

		for dec.More() {
			token, err := dec.Token()
			if err != nil {
				_ = pw.CloseWithError(fmt.Errorf("json stream error: %w", err))
				return
			}

			line, ok := token.(string)
			if !ok {
				_ = pw.CloseWithError(fmt.Errorf("expected string in JSON array, got %T", token))
				return
			}

			if _, err := pw.Write([]byte(line + "\n")); err != nil {
				return
			}
		}

		_, _ = dec.Token()
	}()

	return pr
}
