package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}