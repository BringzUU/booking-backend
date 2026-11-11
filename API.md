# Booking Backend API

Base URL: https://booking-backend-c77p.onrender.com  
Local URL: http://localhost:8080

Use `Content-Type: application/json` for requests with a body. Protected routes
require `Authorization: Bearer <token>` from login. Sessions expire after
24 hours. Logout ends the current session. Changing the password ends all
sessions, so the frontend should clear the token and ask the member to log in.

| Access | Method and path | Request body | Success |
| --- | --- | --- | --- |
| Guest | GET /api/health | | 200 `{"status":"ok"}` |
| Guest | GET /api/rooms | | 200 array of active rooms |
| Guest | GET /api/rooms/{id} | | 200 room |
| Guest | POST /api/auth/register | `name, email, password` | 201 user |
| Guest | POST /api/auth/login | `email, password` | 200 `{token,user}` |
| Member | GET /api/auth/profile | | 200 user |
| Member | POST /api/auth/logout | | 204 |
| Member | PUT /api/auth/password | `current_password, new_password` | 204 |
| Member | GET /api/bookings | | 200 array of own bookings |
| Member | POST /api/bookings | `room_id, booking_date, start_time, end_time, note?` | 201 booking |
| Member | DELETE /api/bookings/{id} | | 204, marks own booking cancelled |
| Admin | GET /api/admin/rooms | | 200 array of active rooms |
| Admin | POST /api/admin/rooms | `name, type, capacity?, description?, image_url?` | 201 room |
| Admin | PUT /api/admin/rooms/{id} | Same fields as POST | 200 room |
| Admin | DELETE /api/admin/rooms/{id} | | 204, hides room from the catalogue |
| Admin | GET /api/admin/bookings | | 200 array of all bookings |
| Admin | PUT /api/admin/bookings/{id} | `status: "approved"` or `"rejected"` | 200 booking |
| Admin | DELETE /api/admin/bookings/{id} | | 204, marks booking cancelled |

Admin accounts can also use member routes. Registration creates a member.
User responses contain `id`, `name`, `email`, and `role`. Room responses
contain `id`, `name`, `type`, `capacity`, `description`, and `image_url`.
The last three fields can be null. A room update replaces the editable fields.

Booking request example. Pick a future date in Thailand time.

```json
{
  "room_id": 1,
  "booking_date": "2027-01-15",
  "start_time": "09:00",
  "end_time": "10:00",
  "note": "???????????"
}
```

The response adds `id`, `user_id`, and `status`. Response times include seconds.
Bookings start as `pending` and reserve the time slot until rejected or
cancelled. Adjacent time slots can be booked separately. An overlapping request
gets 409. Bookings must end on the same date.

Application errors return `{"error":"message"}`. Common statuses are 400 for
invalid input, 401 for a missing or expired session, 403 for a non-admin on an
admin route, 404 for a missing resource, and 409 for a conflict. A 204 response
has an empty body. Empty lists return `[]`.
