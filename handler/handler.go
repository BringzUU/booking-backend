package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	DB *pgxpool.Pool
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/rooms", h.listRooms)
	mux.HandleFunc("GET /api/rooms/{id}", h.getRoom)
	mux.HandleFunc("POST /api/auth/register", h.register)
	mux.HandleFunc("POST /api/auth/login", h.login)
	mux.HandleFunc("POST /api/auth/logout", h.requireUser(false, h.logout))
	mux.HandleFunc("GET /api/auth/profile", h.requireUser(false, h.profile))
	mux.HandleFunc("PUT /api/auth/password", h.requireUser(false, h.changePassword))
	mux.HandleFunc("GET /api/bookings", h.requireUser(false, h.listBookings))
	mux.HandleFunc("POST /api/bookings", h.requireUser(false, h.createBooking))
	mux.HandleFunc("DELETE /api/bookings/{id}", h.requireUser(false, h.cancelBooking))
	mux.HandleFunc("GET /api/admin/rooms", h.requireUser(true, func(w http.ResponseWriter, r *http.Request, _ user, _ []byte) {
		h.listRooms(w, r)
	}))
	mux.HandleFunc("POST /api/admin/rooms", h.requireUser(true, h.saveRoom))
	mux.HandleFunc("PUT /api/admin/rooms/{id}", h.requireUser(true, h.saveRoom))
	mux.HandleFunc("DELETE /api/admin/rooms/{id}", h.requireUser(true, h.deactivateRoom))
	mux.HandleFunc("GET /api/admin/bookings", h.requireUser(true, h.listBookings))
	mux.HandleFunc("PUT /api/admin/bookings/{id}", h.requireUser(true, h.reviewBooking))
	mux.HandleFunc("DELETE /api/admin/bookings/{id}", h.requireUser(true, h.cancelBooking))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func readJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		fail(w, 400, "invalid JSON body")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		fail(w, 400, "expected one JSON object")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		fail(w, 400, "invalid id")
		return 0, false
	}
	return id, true
}

func databaseError(w http.ResponseWriter, operation string, err error) {
	var postgresErr *pgconn.PgError
	if errors.As(err, &postgresErr) {
		log.Printf("%s: PostgreSQL code %s", operation, postgresErr.Code)
	} else {
		log.Printf("%s: %T", operation, err)
	}
	fail(w, 500, "database operation failed")
}

func isUniqueViolation(err error) bool {
	var postgresErr *pgconn.PgError
	return errors.As(err, &postgresErr) && postgresErr.Code == "23505"
}
