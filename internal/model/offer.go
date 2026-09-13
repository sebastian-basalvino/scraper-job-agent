package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Source identifies where an offer was ingested from.
type Source string

const (
	SourceLinkedIn Source = "linkedin"
	SourceWorkana  Source = "workana"
	SourceRemotive Source = "remotive"
)

// Offer is the normalized job offer used across the pipeline.
type Offer struct {
	Source      Source
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
	UniqueID    string
}

// ScoreResult holds the LLM scoring output for an offer.
type ScoreResult struct {
	Summary string
	Score   int
}

// UniqueIDFromLink builds a stable unique ID from a normalized link hash.
func UniqueIDFromLink(link string) string {
	normalized := strings.TrimSpace(strings.ToLower(link))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// UniqueIDFromSourceAndID builds a unique ID from a source-native identifier.
func UniqueIDFromSourceAndID(source Source, nativeID string) string {
	return string(source) + ":" + strings.TrimSpace(nativeID)
}
