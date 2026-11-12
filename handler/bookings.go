package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const bookingColumns = "id, room_id, user_id, booking_date::text, start_time::text, end_time::text, note, status"
const reservedBookingStatus = "status IN ('pending', 'approved', 'confirmed')"

var errRoomTaken = errors.New("room already booked")
var bangkokTime = time.FixedZone("ICT", 7*60*60)

type booking struct {
	ID          int     `json:"id"`
	RoomID      *int    `json:"room_id"`
	UserID      *int    `json:"user_id"`
	BookingDate string  `json:"booking_date"`
	StartTime   string  `json:"start_time"`
	EndTime     string  `json:"end_time"`
	Note        *string `json:"note"`
	Status      string  `json:"status"`
}

func (b *booking) scan(row pgx.Row) error {
	return row.Scan(&b.ID, &b.RoomID, &b.UserID, &b.BookingDate, &b.StartTime, &b.EndTime, &b.Note, &b.Status)
}

func (h *Handler) listBookings(w http.ResponseWriter, r *http.Request, account user, _ []byte) {
	query := "SELECT " + bookingColumns + " FROM public.bookings"
	var args []any
	if r.URL.Path == "/api/bookings" {
		query += " WHERE user_id = $1"
		args = append(args, account.ID)
	} else if r.URL.Query().Has("date") {
		date, ok := bookingDate(w, r)
		if !ok {
			return
		}
		query += " WHERE booking_date = $1::text::date"
		args = append(args, date)
	}
	rows, err := h.DB.Query(r.Context(), query+" ORDER BY booking_date DESC, start_time DESC, id DESC", args...)
	if err != nil {
		databaseError(w, "list bookings", err)
		return
	}
	defer rows.Close()
	bookings := []booking{}
	for rows.Next() {
		var item booking
		if err := item.scan(rows); err != nil {
			databaseError(w, "scan booking", err)
			return
		}
		bookings = append(bookings, item)
	}
	if err := rows.Err(); err != nil {
		databaseError(w, "read bookings", err)
		return
	}
	writeJSON(w, 200, bookings)
}

func (h *Handler) createBooking(w http.ResponseWriter, r *http.Request, account user, _ []byte) {
	var input struct {
		RoomID      int    `json:"room_id"`
		BookingDate string `json:"booking_date"`
		StartTime   string `json:"start_time"`
		EndTime     string `json:"end_time"`
		Note        string `json:"note"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	startsAt, startErr := time.ParseInLocation("2006-01-02 15:04", input.BookingDate+" "+input.StartTime, bangkokTime)
	endsAt, endErr := time.ParseInLocation("2006-01-02 15:04", input.BookingDate+" "+input.EndTime, bangkokTime)
	if input.RoomID < 1 || startErr != nil || endErr != nil ||
		!startsAt.After(time.Now()) || !endsAt.After(startsAt) || len(input.Note) > 500 {
		fail(w, 400, "provide a room and a future booking date with HH:MM start and end times")
		return
	}
	ctx := r.Context()
	var item booking
	err := pgx.BeginTxFunc(ctx, h.DB, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		// Serialize booking requests for the same room until the transaction commits.
		var roomID int
		err := tx.QueryRow(ctx, "SELECT id FROM public.rooms WHERE id = $1 AND is_active = true FOR UPDATE", input.RoomID).Scan(&roomID)
		if err != nil {
			return err
		}
		var occupied bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM public.bookings
			WHERE room_id = $1 AND `+reservedBookingStatus+`
			  AND booking_date = $2::text::date AND start_time < $4::text::time AND end_time > $3::text::time
		)`, roomID, input.BookingDate, input.StartTime, input.EndTime).Scan(&occupied)
		if err != nil {
			return err
		}
		if occupied {
			return errRoomTaken
		}
		return item.scan(tx.QueryRow(ctx, `
			INSERT INTO public.bookings (room_id, user_id, booking_date, start_time, end_time, note, status)
			VALUES ($1, $2, $3::text::date, $4::text::time, $5::text::time, NULLIF($6, ''), 'pending') RETURNING `+bookingColumns,
			roomID, account.ID, input.BookingDate, input.StartTime, input.EndTime, input.Note))
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		fail(w, 404, "room not found")
	case errors.Is(err, errRoomTaken):
		fail(w, 409, "room is already booked during this time")
	case err != nil:
		databaseError(w, "create booking", err)
	default:
		writeJSON(w, 201, item)
	}
}

func (h *Handler) cancelBooking(w http.ResponseWriter, r *http.Request, account user, _ []byte) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	query := "UPDATE public.bookings SET status = 'cancelled' WHERE id = $1 AND status IN ('pending', 'approved', 'confirmed', 'cancelled')"
	args := []any{id}
	if strings.HasPrefix(r.URL.Path, "/api/bookings/") {
		query += " AND user_id = $2"
		args = append(args, account.ID)
	}
	result, err := h.DB.Exec(r.Context(), query, args...)
	if err != nil {
		databaseError(w, "cancel booking", err)
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, 404, "booking not found")
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) reviewBooking(w http.ResponseWriter, r *http.Request, _ user, _ []byte) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if input.Status != "approved" && input.Status != "rejected" {
		fail(w, 400, "status must be approved or rejected")
		return
	}
	var item booking
	err := item.scan(h.DB.QueryRow(r.Context(), `
		UPDATE public.bookings SET status = $2 WHERE id = $1 AND status = 'pending'
		RETURNING `+bookingColumns, id, input.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "pending booking not found")
		return
	}
	if err != nil {
		databaseError(w, "review booking", err)
		return
	}
	writeJSON(w, 200, item)
}
