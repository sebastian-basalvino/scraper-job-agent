package filter

import (
	"testing"

	"scraper/internal/model"
)

func TestShouldDiscard(t *testing.T) {
	offer := model.Offer{
		Title:       "Senior Backend Engineer",
		Description: "Remote Go role with Kafka and AWS in fintech.",
	}
	if ShouldDiscard(offer) {
		t.Fatal("expected relevant offer to pass filter")
	}

	geoOffer := model.Offer{
		Title:       "Backend Engineer",
		Description: "US only role with Go and Kafka.",
	}
	if !ShouldDiscard(geoOffer) {
		t.Fatal("expected US only offer to be discarded")
	}

	onsiteOffer := model.Offer{
		Title:       "Backend Engineer",
		Description: "On-site only role with Go.",
	}
	if !ShouldDiscard(onsiteOffer) {
		t.Fatal("expected onsite offer to be discarded")
	}
}
