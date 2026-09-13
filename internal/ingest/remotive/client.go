package remotive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"scraper/internal/model"
)

const apiURL = "https://remotive.com/api/remote-jobs"

// Client fetches remote job offers from the Remotive public API.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a Remotive API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type apiResponse struct {
	Jobs []apiJob `json:"jobs"`
}

type apiJob struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Publication string `json:"publication_date"`
	Candidate   string `json:"candidate_required_location"`
}

// FetchOffers retrieves and filters remote/LATAM-friendly offers.
func (c *Client) FetchOffers(ctx context.Context) ([]model.Offer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remotive request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remotive status %d", resp.StatusCode)
	}

	var payload apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode remotive response: %w", err)
	}

	var offers []model.Offer
	for _, job := range payload.Jobs {
		if !isLATAMFriendly(job.Candidate) {
			continue
		}

		publishedAt, err := time.Parse(time.RFC3339, job.Publication)
		if err != nil {
			publishedAt = time.Now().UTC()
		}

		offers = append(offers, model.Offer{
			Source:      model.SourceRemotive,
			Title:       job.Title,
			Description: job.Description,
			Link:        job.URL,
			PublishedAt: publishedAt,
			UniqueID:    model.UniqueIDFromSourceAndID(model.SourceRemotive, fmt.Sprintf("%d", job.ID)),
		})
	}

	return offers, nil
}

func isLATAMFriendly(candidateLocation string) bool {
	loc := strings.ToLower(candidateLocation)
	if loc == "" || strings.Contains(loc, "worldwide") || strings.Contains(loc, "anywhere") {
		return true
	}

	latamKeywords := []string{
		"latin america", "latam", "south america", "americas",
		"argentina", "brazil", "chile", "colombia", "mexico", "peru", "uruguay",
	}
	for _, kw := range latamKeywords {
		if strings.Contains(loc, kw) {
			return true
		}
	}

	excluded := []string{"us only", "usa only", "united states only", "eu only", "europe only", "uk only"}
	for _, kw := range excluded {
		if strings.Contains(loc, kw) {
			return false
		}
	}

	return true
}
