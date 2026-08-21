package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"lnk.emsihub.com/internal/database"
	"lnk.emsihub.com/internal/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/lnk.db"
	}

	// Init database
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	// Handlers
	h := handler.New(db)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/shorten", h.Shorten)
	mux.HandleFunc("GET /api/stats/{code}", h.Stats)
	mux.HandleFunc("GET /api/qr/{code}", h.QRCode)

	// Frontend
	mux.HandleFunc("GET /", h.Index)
	mux.HandleFunc("GET /{code}", h.Redirect)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// Static files
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      cors(logging(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("lnk.emsihub.com running on :%s\n", port)
	log.Fatal(srv.ListenAndServe())
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
