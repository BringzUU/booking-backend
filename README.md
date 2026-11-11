# booking-backend

Go API for a room booking web application. See [API.md](API.md) for the
frontend contract.

## Run locally

Use Go 1.27 and set `DATABASE_URL` in an ignored `.env` file or in the process
environment. The database must have the booking project's existing
`public.rooms`, `public.users`, and `public.bookings` tables.
Apply [db/booking_sessions.sql](db/booking_sessions.sql) once to add sessions
and indexes, then run `go run .`. The default port is 8080.

## Render

Set `DATABASE_URL` and set `ALLOWED_ORIGINS` to the frontend origin, such as
`https://your-frontend.vercel.app`. Use comma-separated origins if needed.
Enter the origin without a path or trailing slash. Local React origins on
ports 3000 and 5173 are allowed by default.

To provision an admin, first register a dedicated account through the API,
then update that exact account using the Supabase SQL Editor:

```sql
UPDATE public.users SET role = 'admin' WHERE email = 'replace-with-admin-email@example.com';
```
