package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedRoutesRejectUnauthenticatedRequests(t *testing.T) {
	// A nil DB makes the test fail if a rejected request reaches the database.
	app := (&Handler{}).Routes()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/auth/profile"},
		{http.MethodPut, "/api/auth/password"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/bookings"},
		{http.MethodPost, "/api/bookings"},
		{http.MethodDelete, "/api/bookings/1"},
		{http.MethodGet, "/api/admin/rooms"},
		{http.MethodPost, "/api/admin/rooms"},
		{http.MethodPut, "/api/admin/rooms/1"},
		{http.MethodDelete, "/api/admin/rooms/1"},
		{http.MethodGet, "/api/admin/bookings"},
		{http.MethodPut, "/api/admin/bookings/1"},
		{http.MethodDelete, "/api/admin/bookings/1"},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			for _, authorization := range []string{"", "Basic abc", "Bearer malformed"} {
				req := httptest.NewRequest(route.method, route.path, nil)
				req.Header.Set("Authorization", authorization)
				w := httptest.NewRecorder()
				app.ServeHTTP(w, req)
				if w.Code != http.StatusUnauthorized {
					t.Errorf("authorization %q: got HTTP %d, want 401", authorization, w.Code)
				}
			}
		})
	}
}

func TestStoredPasswordVerification(t *testing.T) {
	const password = "booking-test-password"
	stored := encodePassword(password)
	if stored == password || stored == encodePassword(password) {
		t.Fatal("each stored password should use a fresh salt")
	}
	if !matchesPassword(password, stored) {
		t.Error("correct password was rejected")
	}
	if matchesPassword("wrong-password", stored) {
		t.Error("incorrect password was accepted")
	}
	if matchesPassword(password, "pbkdf2:invalid") {
		t.Error("invalid stored password was accepted")
	}
}
