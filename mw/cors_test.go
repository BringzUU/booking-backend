package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflight(t *testing.T) {
	app := CORS(http.NotFoundHandler(), "https://booking.example, https://preview.example")
	for _, tc := range []struct {
		origin string
		status int
	}{
		{"https://booking.example", http.StatusNoContent},
		{"https://preview.example", http.StatusNoContent},
		{"http://localhost:5173", http.StatusNoContent},
		{"https://booking.example.other.example", http.StatusForbidden},
	} {
		t.Run(tc.origin, func(t *testing.T) {
			req := httptest.NewRequest("OPTIONS", "/api/bookings", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "POST")
			req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("got HTTP %d, want %d", w.Code, tc.status)
			}
			wantOrigin := ""
			if tc.status == http.StatusNoContent {
				wantOrigin = tc.origin
			}
			if origin := w.Header().Get("Access-Control-Allow-Origin"); origin != wantOrigin {
				t.Errorf("got allow-origin %q, want %q", origin, wantOrigin)
			}
		})
	}
}
