package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

const roomColumns = "id, name, type, capacity, description, image_url"

type room struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Capacity    *int    `json:"capacity"`
	Description *string `json:"description"`
	ImageURL    *string `json:"image_url"`
}

func (rm *room) scan(row pgx.Row) error {
	return row.Scan(&rm.ID, &rm.Name, &rm.Type, &rm.Capacity, &rm.Description, &rm.ImageURL)
}

func (h *Handler) listRooms(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), "SELECT "+roomColumns+" FROM public.rooms WHERE is_active = true ORDER BY id")
	if err != nil {
		databaseError(w, "list rooms", err)
		return
	}
	defer rows.Close()
	rooms := []room{}
	for rows.Next() {
		var rm room
		if err := rm.scan(rows); err != nil {
			databaseError(w, "scan room", err)
			return
		}
		rooms = append(rooms, rm)
	}
	if err := rows.Err(); err != nil {
		databaseError(w, "read rooms", err)
		return
	}
	writeJSON(w, 200, rooms)
}

func (h *Handler) getRoom(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var rm room
	err := rm.scan(h.DB.QueryRow(r.Context(), "SELECT "+roomColumns+" FROM public.rooms WHERE id = $1 AND is_active = true", id))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "room not found")
		return
	}
	if err != nil {
		databaseError(w, "get room", err)
		return
	}
	writeJSON(w, 200, rm)
}

func (h *Handler) saveRoom(w http.ResponseWriter, r *http.Request, _ user, _ []byte) {
	var input struct {
		Name        string  `json:"name"`
		Type        string  `json:"type"`
		Capacity    *int    `json:"capacity"`
		Description *string `json:"description"`
		ImageURL    *string `json:"image_url"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Type = strings.TrimSpace(input.Type)
	if input.Name == "" || input.Type == "" || (input.Capacity != nil && *input.Capacity < 1) {
		fail(w, 400, "name and type are required, and capacity must be positive when provided")
		return
	}
	query := "INSERT INTO public.rooms (name, type, capacity, description, image_url, is_active) VALUES ($1, $2, $3, $4, $5, true)"
	args := []any{input.Name, input.Type, input.Capacity, input.Description, input.ImageURL}
	status := 201
	if r.Method == http.MethodPut {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		query = "UPDATE public.rooms SET name = $1, type = $2, capacity = $3, description = $4, image_url = $5 WHERE id = $6 AND is_active = true"
		args = append(args, id)
		status = 200
	}
	var rm room
	err := rm.scan(h.DB.QueryRow(r.Context(), query+" RETURNING "+roomColumns, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "room not found")
		return
	}
	if err != nil {
		databaseError(w, "save room", err)
		return
	}
	writeJSON(w, status, rm)
}

func (h *Handler) deactivateRoom(w http.ResponseWriter, r *http.Request, _ user, _ []byte) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	result, err := h.DB.Exec(r.Context(), "UPDATE public.rooms SET is_active = false WHERE id = $1 AND is_active = true", id)
	if err != nil {
		databaseError(w, "deactivate room", err)
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, 404, "room not found")
		return
	}
	w.WriteHeader(204)
}
