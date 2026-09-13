package imap

import (
	"testing"
	"time"
)

const workanaFixture = `
<html>
<body>
  <a href="https://www.workana.com/job/backend-go-developer">Backend Go Developer</a>
  <p>Remote project with Kafka and AWS.</p>
</body>
</html>
`

func TestParseWorkana(t *testing.T) {
	offers, err := ParseWorkana(workanaFixture, time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parse workana: %v", err)
	}
	if len(offers) != 1 {
		t.Fatalf("expected 1 offer, got %d", len(offers))
	}
	if offers[0].Title != "Backend Go Developer" {
		t.Fatalf("unexpected title: %s", offers[0].Title)
	}
	if offers[0].Link != "https://www.workana.com/job/backend-go-developer" {
		t.Fatalf("unexpected link: %s", offers[0].Link)
	}
}
