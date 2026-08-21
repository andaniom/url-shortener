package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type URL struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Original  string    `json:"original"`
	Clicks    int64     `json:"clicks"`
	CreatedAt time.Time `json:"created_at"`
}

type Stats struct {
	URL
	RecentClicks []Click `json:"recent_clicks"`
}

type Click struct {
	ID        int64     `json:"id"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	CreatedAt time.Time `json:"created_at"`
}

func New(dbPath string) (*DB, error) {
	// Create directory if not exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Create tables
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() {
	db.conn.Close()
}

func migrate(conn *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT UNIQUE NOT NULL,
			original TEXT NOT NULL,
			clicks INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_urls_code ON urls(code)`,
		`CREATE TABLE IF NOT EXISTS clicks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url_id INTEGER NOT NULL,
			ip TEXT,
			user_agent TEXT,
			referer TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (url_id) REFERENCES urls(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_clicks_url_id ON clicks(url_id)`,
	}

	for _, q := range queries {
		if _, err := conn.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) CreateURL(code, original string) (*URL, error) {
	result, err := db.conn.Exec(
		"INSERT INTO urls (code, original) VALUES (?, ?)",
		code, original,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &URL{
		ID:       id,
		Code:     code,
		Original: original,
		Clicks:   0,
	}, nil
}

func (db *DB) GetURL(code string) (*URL, error) {
	var u URL
	err := db.conn.QueryRow(
		"SELECT id, code, original, clicks, created_at FROM urls WHERE code = ?",
		code,
	).Scan(&u.ID, &u.Code, &u.Original, &u.Clicks, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) IncrementClicks(code string) error {
	_, err := db.conn.Exec(
		"UPDATE urls SET clicks = clicks + 1 WHERE code = ?",
		code,
	)
	return err
}

func (db *DB) RecordClick(urlID int64, ip, userAgent, referer string) error {
	_, err := db.conn.Exec(
		"INSERT INTO clicks (url_id, ip, user_agent, referer) VALUES (?, ?, ?, ?)",
		urlID, ip, userAgent, referer,
	)
	return err
}

func (db *DB) GetStats(code string) (*Stats, error) {
	u, err := db.GetURL(code)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(
		"SELECT id, ip, user_agent, referer, created_at FROM clicks WHERE url_id = ? ORDER BY created_at DESC LIMIT 10",
		u.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clicks []Click
	for rows.Next() {
		var c Click
		rows.Scan(&c.ID, &c.IP, &c.UserAgent, &c.Referer, &c.CreatedAt)
		clicks = append(clicks, c)
	}

	return &Stats{
		URL:          *u,
		RecentClicks: clicks,
	}, nil
}

func (db *DB) CodeExists(code string) bool {
	var exists bool
	db.conn.QueryRow("SELECT EXISTS(SELECT 1 FROM urls WHERE code = ?)", code).Scan(&exists)
	return exists
}
