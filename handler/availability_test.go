package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAvailabilityRejectsInvalidInput(t *testing.T) {
	app := (&Handler{}).Routes()
	for _, path := range []string{
		"/api/rooms/0/availability?date=2027-01-15",
		"/api/rooms/8/availability",
		"/api/rooms/8/availability?date=2027-02-29",
		"/api/rooms/8/availability?date=0000-01-01",
		"/api/rooms/8/availability?date=2027-01-15&date=2027-01-16",
	} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusBadRequest {
				t.Errorf("got HTTP %d, want 400", w.Code)
			}
		})
	}
}
