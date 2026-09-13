package imap

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"scraper/internal/model"
)

var (
	workanaLinkRe = regexp.MustCompile(`https?://(?:www\.)?workana\.com/[^\s"'<>]+`)
)

// ParseWorkana extracts job offers from a Workana alert email body.
func ParseWorkana(body string, receivedAt time.Time) ([]model.Offer, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return parseWorkanaPlainText(body, receivedAt)
	}

	var offers []model.Offer
	doc.Find("a").Each(func(_ int, sel *goquery.Selection) {
		link, ok := sel.Attr("href")
		if !ok || !strings.Contains(link, "workana.com") {
			return
		}

		title := strings.TrimSpace(sel.Text())
		if title == "" {
			return
		}

		description := strings.TrimSpace(sel.Parent().Text())
		offers = append(offers, model.Offer{
			Source:      model.SourceWorkana,
			Title:       title,
			Description: description,
			Link:        link,
			PublishedAt: receivedAt,
			UniqueID:    model.UniqueIDFromLink(link),
		})
	})

	if len(offers) > 0 {
		return dedupeOffers(offers), nil
	}

	return parseWorkanaPlainText(body, receivedAt)
}

func parseWorkanaPlainText(body string, receivedAt time.Time) ([]model.Offer, error) {
	links := workanaLinkRe.FindAllString(body, -1)
	if len(links) == 0 {
		return nil, fmt.Errorf("no workana links found")
	}

	var offers []model.Offer
	for _, link := range links {
		offers = append(offers, model.Offer{
			Source:      model.SourceWorkana,
			Title:       "Workana project",
			Description: body,
			Link:        link,
			PublishedAt: receivedAt,
			UniqueID:    model.UniqueIDFromLink(link),
		})
	}
	return dedupeOffers(offers), nil
}

func dedupeOffers(offers []model.Offer) []model.Offer {
	seen := make(map[string]bool)
	var result []model.Offer
	for _, offer := range offers {
		if seen[offer.UniqueID] {
			continue
		}
		seen[offer.UniqueID] = true
		result = append(result, offer)
	}
	return result
}
