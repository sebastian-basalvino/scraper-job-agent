package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"scraper/internal/model"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS offers (
    unique_id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    title TEXT NOT NULL,
    link TEXT NOT NULL,
    processed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    score INTEGER,
    notified BOOLEAN NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_offers_processed_at ON offers(processed_at);
`

// Store persists processed offers for deduplication.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database and applies the schema.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// IsSeen returns true if the offer unique ID was already processed.
func (s *Store) IsSeen(uniqueID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM offers WHERE unique_id = ?`, uniqueID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check seen: %w", err)
	}
	return count > 0, nil
}

// Save persists an offer with optional score and notification flag.
func (s *Store) Save(offer model.Offer, score *int, notified bool) error {
	_, err := s.db.Exec(
		`INSERT INTO offers (unique_id, source, title, link, processed_at, score, notified)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(unique_id) DO UPDATE SET
		   source = excluded.source,
		   title = excluded.title,
		   link = excluded.link,
		   processed_at = excluded.processed_at,
		   score = excluded.score,
		   notified = excluded.notified`,
		offer.UniqueID,
		string(offer.Source),
		offer.Title,
		offer.Link,
		time.Now().UTC(),
		score,
		notified,
	)
	if err != nil {
		return fmt.Errorf("save offer: %w", err)
	}
	return nil
}
