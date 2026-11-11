package handler

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
)

type user struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func passwordHash(password string, salt []byte) []byte {
	hash, err := pbkdf2.Key(sha256.New, password, salt, 600000, 32)
	if err != nil {
		panic(err)
	}
	return hash
}

func validPassword(password string) bool {
	return len(password) >= 8 && len(password) <= 128
}

func encodePassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	return "pbkdf2:" + base64.RawStdEncoding.EncodeToString(append(salt, passwordHash(password, salt)...))
}

func matchesPassword(password, stored string) bool {
	salt, hash := make([]byte, 16), make([]byte, 32)
	encoded, hasPrefix := strings.CutPrefix(stored, "pbkdf2:")
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	valid := hasPrefix && err == nil && len(raw) == 48
	if valid {
		salt, hash = raw[:16], raw[16:]
	}
	matches := subtle.ConstantTimeCompare(passwordHash(password, salt), hash)
	return valid && matches == 1
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 ||
		input.Name == "" || len(input.Name) > 200 || !validPassword(input.Password) {
		fail(w, 400, "provide a name, email and password of 8 to 128 bytes")
		return
	}
	var account user
	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO public.users (name, email, password_hash, role)
		VALUES ($1, $2, $3, 'member') RETURNING id, name, email, role`,
		input.Name, input.Email, encodePassword(input.Password)).
		Scan(&account.ID, &account.Name, &account.Email, &account.Role)
	if isUniqueViolation(err) {
		fail(w, 409, "email already registered")
		return
	}
	if err != nil {
		databaseError(w, "register", err)
		return
	}
	writeJSON(w, 201, account)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if !validPassword(input.Password) {
		fail(w, 401, "invalid email or password")
		return
	}
	ctx := r.Context()
	var account user
	var hash string
	err := h.DB.QueryRow(ctx, `
		SELECT id, name, email, role, password_hash FROM public.users WHERE lower(email) = $1`,
		strings.ToLower(strings.TrimSpace(input.Email))).
		Scan(&account.ID, &account.Name, &account.Email, &account.Role, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		databaseError(w, "login", err)
		return
	}
	if !matchesPassword(input.Password, hash) || err != nil {
		fail(w, 401, "invalid email or password")
		return
	}
	token := make([]byte, 32)
	rand.Read(token)
	digest := sha256.Sum256(token)
	result, err := h.DB.Exec(ctx, `
		INSERT INTO booking_private.sessions (token_hash, user_id, expires_at)
		SELECT $1, id, now() + interval '24 hours'
		FROM public.users WHERE id = $2 AND password_hash = $3 FOR UPDATE`,
		digest[:], account.ID, hash)
	if err != nil {
		databaseError(w, "create session", err)
		return
	}
	if result.RowsAffected() == 0 {
		fail(w, 401, "invalid email or password")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"token": base64.RawURLEncoding.EncodeToString(token), "user": account})
}

type authenticatedHandler func(http.ResponseWriter, *http.Request, user, []byte)

func (h *Handler) requireUser(adminOnly bool, next authenticatedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			fail(w, 401, "login required")
			return
		}
		token, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil || len(token) != 32 {
			fail(w, 401, "invalid session")
			return
		}
		digest := sha256.Sum256(token)
		var account user
		err = h.DB.QueryRow(r.Context(), `
			SELECT u.id, u.name, u.email, u.role FROM booking_private.sessions s
			JOIN public.users u ON u.id = s.user_id
			WHERE s.token_hash = $1 AND s.expires_at > now()`, digest[:]).
			Scan(&account.ID, &account.Name, &account.Email, &account.Role)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 401, "invalid or expired session")
			return
		}
		if err != nil {
			databaseError(w, "check session", err)
			return
		}
		if adminOnly && account.Role != "admin" {
			fail(w, 403, "admin access required")
			return
		}
		next(w, r, account, digest[:])
	}
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request, account user, _ []byte) {
	writeJSON(w, 200, account)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, _ user, token []byte) {
	if _, err := h.DB.Exec(r.Context(), "DELETE FROM booking_private.sessions WHERE token_hash = $1", token); err != nil {
		databaseError(w, "logout", err)
		return
	}
	w.WriteHeader(204)
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request, account user, _ []byte) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	if !validPassword(input.CurrentPassword) || !validPassword(input.NewPassword) {
		fail(w, 400, "password must be 8 to 128 bytes")
		return
	}
	ctx := r.Context()
	var hash string
	if err := h.DB.QueryRow(ctx, "SELECT password_hash FROM public.users WHERE id = $1", account.ID).Scan(&hash); err != nil {
		databaseError(w, "read password", err)
		return
	}
	if !matchesPassword(input.CurrentPassword, hash) {
		fail(w, 400, "incorrect current password")
		return
	}
	newHash := encodePassword(input.NewPassword)
	err := pgx.BeginFunc(ctx, h.DB, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE public.users SET password_hash = $1
			WHERE id = $2 AND password_hash = $3`, newHash, account.ID, hash)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		_, err = tx.Exec(ctx, "DELETE FROM booking_private.sessions WHERE user_id = $1", account.ID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 409, "password changed concurrently")
		return
	}
	if err != nil {
		databaseError(w, "change password", err)
		return
	}
	w.WriteHeader(204)
}
