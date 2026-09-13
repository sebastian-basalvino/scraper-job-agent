package filter

import (
	"regexp"
	"strings"

	"github.com/sebastian-basalvino/scraper-job-agent/internal/model"
)

var (
	geoExcludePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(us only|usa only|united states only|eu only|europe only|uk only|canada only)\b`),
		regexp.MustCompile(`(?i)\b(no\s+latam|latin america not|not accepting applications from latin america)\b`),
	}

	nonRemotePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(on[- ]site|onsite only|hybrid required|office[- ]based|in[- ]person only)\b`),
	}

	stackKeywords = []string{
		"go", "golang", "kotlin", "spring boot", "spring", "c#", ".net",
		"kafka", "sqs", "sns", "distributed", "microservices",
		"aws", "postgres", "postgresql", "oracle", "redis", "dynamodb",
		"fintech", "payments", "payment", "remittance", "iso 20022", "swift",
		"backend", "back-end", "back end",
	}
)

// ShouldDiscard returns true when an offer should be filtered out before LLM scoring.
func ShouldDiscard(offer model.Offer) bool {
	text := strings.ToLower(offer.Title + " " + offer.Description)

	for _, pattern := range geoExcludePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}

	for _, pattern := range nonRemotePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}

	return !hasStackMatch(text)
}

func hasStackMatch(text string) bool {
	for _, kw := range stackKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
