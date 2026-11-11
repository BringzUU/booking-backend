package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBookingFlow(t *testing.T) {
	dsn := os.Getenv("BOOKING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set BOOKING_TEST_DATABASE_URL to a prepared test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("invalid test database connection string")
	}
	defer db.Close()

	suffix := fmt.Sprint(time.Now().UnixNano())
	email := "member-" + suffix + "@example.test"
	adminEmail := "admin-" + suffix + "@example.test"
	otherEmail := "other-" + suffix + "@example.test"
	var roomID int
	if err := db.QueryRow(ctx, `
		INSERT INTO public.rooms (name, type, capacity, is_active)
		VALUES ($1, 'test', 4, true) RETURNING id`, "Test room "+suffix).Scan(&roomID); err != nil {
		t.Fatalf("create test room: %T", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		statements := []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM public.bookings WHERE room_id = $1", []any{roomID}},
			{"DELETE FROM public.rooms WHERE id = $1", []any{roomID}},
			{"DELETE FROM public.users WHERE email IN ($1, $2, $3)", []any{email, adminEmail, otherEmail}},
		}
		for _, statement := range statements {
			if _, err := db.Exec(cleanupCtx, statement.sql, statement.args...); err != nil {
				t.Errorf("could not clean test fixtures: %v", err)
			}
		}
	}()
	app := (&Handler{DB: db}).Routes()
	request := func(method, path, token string, body any) *httptest.ResponseRecorder {
		data, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(data)).WithContext(ctx)
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, r)
		return w
	}
	want := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("got HTTP %d, want %d", w.Code, status)
		}
	}
	register := func(address, password string) {
		t.Helper()
		want(request("POST", "/api/auth/register", "", map[string]string{
			"name": "Integration Test", "email": address, "password": password,
		}), 201)
	}
	login := func(address, password string) string {
		t.Helper()
		w := request("POST", "/api/auth/login", "", map[string]string{"email": address, "password": password})
		want(w, 200)
		var result struct{ Token string }
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Token == "" {
			t.Fatal("login did not return a token")
		}
		return result.Token
	}

	const password = "test-password-2026"
	const newPassword = "test-password-updated"
	want(request("GET", fmt.Sprintf("/api/rooms/%d", roomID), "", nil), 200)
	want(request("GET", "/api/bookings", "", nil), 401)
	register(email, password)
	register(adminEmail, password)
	register(otherEmail, password)
	if _, err := db.Exec(ctx, "UPDATE public.users SET role = 'admin' WHERE email = $1", adminEmail); err != nil {
		t.Fatalf("set fixture admin role: %T", err)
	}
	memberToken := login(email, password)
	adminToken := login(adminEmail, password)
	otherToken := login(otherEmail, password)
	want(request("GET", "/api/auth/profile", memberToken, nil), 200)
	want(request("GET", "/api/admin/bookings", memberToken, nil), 403)

	input := map[string]any{
		"room_id": roomID, "booking_date": time.Now().In(bangkokTime).AddDate(0, 0, 2).Format("2006-01-02"),
		"start_time": "09:00", "end_time": "10:00",
	}
	var responses [2]*httptest.ResponseRecorder
	var group sync.WaitGroup
	for i, token := range []string{memberToken, otherToken} {
		group.Go(func() { responses[i] = request("POST", "/api/bookings", token, input) })
	}
	group.Wait()
	winner := 0
	if responses[1].Code == 201 {
		winner = 1
	}
	want(responses[winner], 201)
	want(responses[1-winner], 409)
	var item booking
	if err := json.Unmarshal(responses[winner].Body.Bytes(), &item); err != nil || item.ID < 1 || item.Status != "pending" {
		t.Fatal("expected a pending booking")
	}
	tokens := []string{memberToken, otherToken}
	memberPath := fmt.Sprintf("/api/bookings/%d", item.ID)
	adminPath := fmt.Sprintf("/api/admin/bookings/%d", item.ID)
	own := request("GET", "/api/bookings", tokens[winner], nil)
	want(own, 200)
	var ownBookings []booking
	if err := json.Unmarshal(own.Body.Bytes(), &ownBookings); err != nil || len(ownBookings) != 1 || ownBookings[0].ID != item.ID {
		t.Fatal("owner should see the booking")
	}
	other := request("GET", "/api/bookings", tokens[1-winner], nil)
	want(other, 200)
	if other.Body.String() != "[]\n" {
		t.Fatal("another member can see the booking")
	}
	want(request("DELETE", memberPath, tokens[1-winner], nil), 404)
	want(request("PUT", adminPath, adminToken, map[string]string{"status": "approved"}), 200)
	want(request("DELETE", memberPath, tokens[winner], nil), 204)
	want(request("POST", "/api/bookings", tokens[1-winner], input), 201)

	want(request("PUT", "/api/auth/password", memberToken, map[string]string{
		"current_password": password, "new_password": newPassword,
	}), 204)
	want(request("GET", "/api/auth/profile", memberToken, nil), 401)
	want(request("POST", "/api/auth/login", "", map[string]string{"email": email, "password": password}), 401)
	memberToken = login(email, newPassword)
	want(request("POST", "/api/auth/logout", memberToken, nil), 204)
	want(request("GET", "/api/auth/profile", memberToken, nil), 401)
}
