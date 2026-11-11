CREATE SCHEMA IF NOT EXISTS booking_private;

CREATE TABLE IF NOT EXISTS booking_private.sessions (
    token_hash bytea PRIMARY KEY,
    user_id integer NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL
);

ALTER TABLE booking_private.sessions ENABLE ROW LEVEL SECURITY;

CREATE INDEX IF NOT EXISTS booking_sessions_user_id_idx
    ON booking_private.sessions (user_id);

CREATE UNIQUE INDEX IF NOT EXISTS booking_users_email_lower_idx
    ON public.users (lower(email));

CREATE INDEX IF NOT EXISTS booking_bookings_user_id_idx
    ON public.bookings (user_id);

CREATE INDEX IF NOT EXISTS booking_bookings_room_date_idx
    ON public.bookings (room_id, booking_date, start_time)
    WHERE status IN ('pending', 'approved', 'confirmed');
