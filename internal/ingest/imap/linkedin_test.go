package imap

import (
	"testing"
	"time"
)

const linkedInFixture = `
<html>
<body>
  <div class="job-card">
    <a href="https://www.linkedin.com/jobs/view/123456">Senior Backend Engineer (Go)</a>
    <p>Remote fintech role with Kafka and AWS.</p>
  </div>
  <div class="job-card">
    <a href="https://www.linkedin.com/jobs/view/789012">Kotlin Platform Engineer</a>
    <p>Distributed systems and payments.</p>
  </div>
</body>
</html>
`

func TestParseLinkedIn(t *testing.T) {
	offers, err := ParseLinkedIn(linkedInFixture, time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parse linkedin: %v", err)
	}
	if len(offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(offers))
	}
	if offers[0].Title != "Senior Backend Engineer (Go)" {
		t.Fatalf("unexpected first title: %s", offers[0].Title)
	}
}
