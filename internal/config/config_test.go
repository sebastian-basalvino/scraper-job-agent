package config

import (
	"testing"

	"github.com/sebastian-basalvino/scraper-job-agent/internal/model"
)

func TestParseEnabledSources(t *testing.T) {
	enabled, err := parseEnabledSources("linkedin,remotive")
	if err != nil {
		t.Fatalf("parse enabled sources: %v", err)
	}
	if !enabled[model.SourceLinkedIn] || !enabled[model.SourceRemotive] {
		t.Fatalf("expected linkedin and remotive enabled, got %#v", enabled)
	}
	if enabled[model.SourceWorkana] {
		t.Fatal("expected workana disabled")
	}
}

func TestParseEnabledSourcesRejectsUnknown(t *testing.T) {
	_, err := parseEnabledSources("linkedin,foo")
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestParseEnabledSourcesRequiresAtLeastOne(t *testing.T) {
	_, err := parseEnabledSources(" , ")
	if err == nil {
		t.Fatal("expected error for empty enabled sources")
	}
}
