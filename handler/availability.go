package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

type occupiedPeriod struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type availability struct {
	RoomID      int              `json:"room_id"`
	BookingDate string           `json:"booking_date"`
	Occupied    []occupiedPeriod `json:"occupied"`
}

func bookingDate(w http.ResponseWriter, r *http.Request) (string, bool) {
	dates := r.URL.Query()["date"]
	if len(dates) == 1 {
		date, err := time.Parse(time.DateOnly, dates[0])
		if err == nil && date.Year() > 0 {
			return dates[0], true
		}
	}
	fail(w, 400, "provide one date in YYYY-MM-DD format")
	return "", false
}

func (h *Handler) roomAvailability(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	date, ok := bookingDate(w, r)
	if !ok {
		return
	}

	result := availability{RoomID: id, BookingDate: date}
	err := h.DB.QueryRow(r.Context(), `
		SELECT (
			SELECT COALESCE(json_agg(
				json_build_object('start_time', start_time::text, 'end_time', end_time::text)
				ORDER BY start_time, end_time
			), '[]'::json)
			FROM public.bookings
			WHERE room_id = r.id AND booking_date = $2::text::date
			  AND `+reservedBookingStatus+`
		)
		FROM public.rooms r WHERE r.id = $1 AND r.is_active = true`, id, date).Scan(&result.Occupied)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "room not found")
		return
	}
	if err != nil {
		databaseError(w, "read room availability", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}
