package store

import (
	"path/filepath"
	"testing"
	"time"

	"scraper/internal/model"
)

func TestStoreIsSeenAndSave(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "offers.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	offer := model.Offer{
		Source:      model.SourceWorkana,
		Title:       "Backend Go Developer",
		Description: "Remote backend role",
		Link:        "https://workana.com/job/123",
		PublishedAt: time.Now().UTC(),
		UniqueID:    model.UniqueIDFromLink("https://workana.com/job/123"),
	}

	seen, err := s.IsSeen(offer.UniqueID)
	if err != nil {
		t.Fatalf("is seen: %v", err)
	}
	if seen {
		t.Fatal("expected offer to be unseen")
	}

	score := 72
	if err := s.Save(offer, &score, true); err != nil {
		t.Fatalf("save offer: %v", err)
	}

	seen, err = s.IsSeen(offer.UniqueID)
	if err != nil {
		t.Fatalf("is seen after save: %v", err)
	}
	if !seen {
		t.Fatal("expected offer to be seen after save")
	}
}
