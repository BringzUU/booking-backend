package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"booking-backend/handler"
	"booking-backend/mw"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatal("invalid database configuration")
	}
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	app := &handler.Handler{DB: db}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mw.CORS(app.Routes(), os.Getenv("ALLOWED_ORIGINS")),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
	}
	log.Printf("server running on port %s", port)
	log.Fatal(server.ListenAndServe())
}
