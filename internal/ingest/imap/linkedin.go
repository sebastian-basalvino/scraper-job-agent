package imap

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"scraper/internal/model"
)

var linkedInJobLinkRe = regexp.MustCompile(`https?://(?:www\.)?linkedin\.com/[^\s"'<>]+`)

// ParseLinkedIn extracts multiple job offers from a LinkedIn alert email body.
func ParseLinkedIn(body string, receivedAt time.Time) ([]model.Offer, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return parseLinkedInPlainText(body, receivedAt)
	}

	var offers []model.Offer
	doc.Find("a").Each(func(_ int, sel *goquery.Selection) {
		link, ok := sel.Attr("href")
		if !ok {
			return
		}
		if !strings.Contains(link, "linkedin.com/jobs") && !strings.Contains(link, "linkedin.com/comm/jobs") {
			return
		}

		title := strings.TrimSpace(sel.Text())
		if title == "" || strings.EqualFold(title, "view job") {
			return
		}

		description := strings.TrimSpace(sel.Parent().Parent().Text())
		if description == "" {
			description = strings.TrimSpace(sel.Parent().Text())
		}

		offers = append(offers, model.Offer{
			Source:      model.SourceLinkedIn,
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

	return parseLinkedInPlainText(body, receivedAt)
}

func parseLinkedInPlainText(body string, receivedAt time.Time) ([]model.Offer, error) {
	links := linkedInJobLinkRe.FindAllString(body, -1)
	if len(links) == 0 {
		return nil, fmt.Errorf("no linkedin job links found")
	}

	var offers []model.Offer
	for _, link := range links {
		if !strings.Contains(link, "/jobs") {
			continue
		}
		offers = append(offers, model.Offer{
			Source:      model.SourceLinkedIn,
			Title:       "LinkedIn job alert",
			Description: body,
			Link:        link,
			PublishedAt: receivedAt,
			UniqueID:    model.UniqueIDFromLink(link),
		})
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no linkedin job links found")
	}
	return dedupeOffers(offers), nil
}
