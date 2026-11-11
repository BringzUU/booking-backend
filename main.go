package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type Room struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Capacity    *int    `json:"capacity"`
	Description *string `json:"description"`
	ImageURL    *string `json:"image_url"`
}

var db *pgxpool.Pool

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func main() {
	godotenv.Load() // อ่าน .env ในเครื่อง (บน Render ไม่มีไฟล์นี้ ไม่เป็นไร)

	var err error
	db, err = pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("เชื่อมฐานข้อมูลไม่ได้: ", err)
	}
	defer db.Close()

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	http.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(context.Background(),
			"SELECT id, name, type, capacity, description, image_url FROM rooms WHERE is_active = true ORDER BY id")
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "ดึงข้อมูลไม่สำเร็จ"})
			return
		}
		defer rows.Close()

		rooms := []Room{}
		for rows.Next() {
			var rm Room
			if err := rows.Scan(&rm.ID, &rm.Name, &rm.Type, &rm.Capacity, &rm.Description, &rm.ImageURL); err != nil {
				writeJSON(w, 500, map[string]string{"error": "อ่านข้อมูลไม่สำเร็จ"})
				return
			}
			rooms = append(rooms, rm)
		}
		writeJSON(w, 200, rooms)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
