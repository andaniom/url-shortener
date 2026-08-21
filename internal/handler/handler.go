package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode/qrcode"

	"lnk.emsihub.com/internal/database"
)

type Handler struct {
	db *database.DB
}

func New(db *database.DB) *Handler {
	return &Handler{db: db}
}

// Request/Response types
type ShortenRequest struct {
	URL  string `json:"url"`
	Code string `json:"code,omitempty"`
}

type ShortenResponse struct {
	ShortURL string `json:"short_url"`
	Code     string `json:"code"`
	Original string `json:"original"`
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate URL
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		req.URL = "https://" + req.URL
	}

	// Generate or validate code
	code := req.Code
	if code == "" {
		code = h.generateCode()
	} else {
		// Validate custom code
		if len(code) < 3 || len(code) > 20 {
			http.Error(w, "Code must be 3-20 characters", http.StatusBadRequest)
			return
		}
		if h.db.CodeExists(code) {
			http.Error(w, "Code already exists", http.StatusConflict)
			return
		}
	}

	// Save
	url, err := h.db.CreateURL(code, req.URL)
	if err != nil {
		http.Error(w, "Failed to create short URL", http.StatusInternalServerError)
		return
	}

	// Get host
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}

	resp := ShortenResponse{
		ShortURL: fmt.Sprintf("https://%s/%s", host, url.Code),
		Code:     url.Code,
		Original: url.Original,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	url, err := h.db.GetURL(code)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Record click
	h.db.IncrementClicks(code)
	h.db.RecordClick(url.ID, r.RemoteAddr, r.UserAgent(), r.Referer())

	http.Redirect(w, r, url.Original, http.StatusMovedPermanently)
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	stats, err := h.db.GetStats(code)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) QRCode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	// Verify URL exists
	_, err := h.db.GetURL(code)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}

	url := fmt.Sprintf("https://%s/%s", host, code)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl := template.Must(template.ParseFiles("static/index.html"))
	tmpl.Execute(w, nil)
}

func (h *Handler) generateCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	code := base64.URLEncoding.EncodeToString(b)[:6]

	// Ensure unique
	if h.db.CodeExists(code) {
		return h.generateCode()
	}
	return code
}

// randomString generates a random string of given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		result[i] = letters[num.Int64()]
	}
	return string(result)
}
