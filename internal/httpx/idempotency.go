package httpx

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/changerr/dishflow-grok/internal/app"
	"github.com/changerr/dishflow-grok/internal/cryptoutil"
)

type captureWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
}

func (c *captureWriter) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *captureWriter) Write(b []byte) (int, error) {
	c.buf.Write(b)
	return c.ResponseWriter.Write(b)
}

func (s *Server) idempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		path := r.URL.Path
		if strings.HasSuffix(path, "/admin/session") || strings.HasSuffix(path, "/auth/wechat/session") {
			next.ServeHTTP(w, r)
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		r.Body = io.NopCloser(bytes.NewReader(body))
		subject := idemSubject(r, s.App.CookieName())
		reqHash := app.HashRequest(map[string]any{"method": r.Method, "path": path, "body": string(body)})
		st, saved, ok, err := s.App.IdempotencyGet(r.Context(), subject, key, reqHash)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if ok {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(st)
			_, _ = w.Write(saved)
			return
		}
		cap := &captureWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(cap, r)
		if cap.status >= 200 && cap.status < 500 {
			_ = s.App.IdempotencyPut(r.Context(), subject, key, reqHash, cap.status, cap.buf.Bytes())
		}
	})
}

func idemSubject(r *http.Request, cookie string) string {
	if c, err := r.Cookie(cookie); err == nil && c.Value != "" {
		return "admin:" + cryptoutil.HashToken(c.Value)
	}
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if auth != "" {
		return "cust:" + cryptoutil.HashToken(auth)
	}
	return "anon:" + r.RemoteAddr
}
