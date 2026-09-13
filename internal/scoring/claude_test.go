package scoring

import (
	"testing"
)

func TestParseScoreResponse(t *testing.T) {
	text := `RESUMEN: Rol remoto de backend en Go con Kafka y fintech.
PUNTUACIÓN: 78`

	result, err := parseScoreResponse(text)
	if err != nil {
		t.Fatalf("parse score response: %v", err)
	}
	if result.Score != 78 {
		t.Fatalf("unexpected score: %d", result.Score)
	}
	if result.Summary == "" {
		t.Fatal("expected summary")
	}
}
