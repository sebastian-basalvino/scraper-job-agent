package scoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"scraper/internal/model"
)

const systemPrompt = `Eres un asistente que evalúa ofertas de trabajo freelance/remoto para un backend engineer senior con este perfil:

- 5-8 años de experiencia, foco en backend (Go, Kotlin/Spring Boot, C# en el pasado)
- Sistemas distribuidos, alta transaccionalidad, mensajería (Kafka, SQS/SNS)
- Fintech: pagos, transferencias, cash-in, remesas, normas ISO 20022/SWIFT
- Cloud: AWS (EC2, S3, DynamoDB), bases relacionales y NoSQL (Postgres, Oracle, Redis)
- Experiencia liderando equipos técnicos, pero busca roles hands-on con código
- Preferencia: modalidad remota, sin restricción geográfica que excluya LATAM

Analiza la siguiente oferta de trabajo y respondé en este formato exacto:

RESUMEN: [2-3 líneas describiendo de qué se trata la oferta y por qué encaja o no con el perfil]
PUNTUACIÓN: [número del 1 al 100]

Guía de referencia para la puntuación:
0-20 = No relevante (stack o dominio totalmente distinto)
21-40 = Poco relevante (algún match menor)
41-64 = Relevante pero con dudas (buen match de stack, pero falla en modalidad/geografía/seniority)
65-84 = Muy relevante (match fuerte en stack + modalidad + geografía)
85-100 = Ideal (match fuerte + fintech o dominio similar a su experiencia)`

var (
	summaryRe = regexp.MustCompile(`(?i)RESUMEN:\s*(.+?)(?:\n|PUNTUACI[ÓO]N:)`)
	scoreRe   = regexp.MustCompile(`(?i)PUNTUACI[ÓO]N:\s*(\d{1,3})`)
)

// Client scores offers using the Anthropic Claude API.
type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewClient creates a Claude scoring client.
func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Score evaluates an offer and returns summary and score.
func (c *Client) Score(ctx context.Context, offer model.Offer) (model.ScoreResult, error) {
	userPrompt := fmt.Sprintf("%s\n\nOferta:\n%s", systemPrompt, offerText(offer))

	reqBody := messagesRequest{
		Model:     c.model,
		MaxTokens: 300,
		System:    "Respondé únicamente con el formato solicitado.",
		Messages: []message{
			{Role: "user", Content: userPrompt},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return model.ScoreResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return model.ScoreResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.ScoreResult{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.ScoreResult{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return model.ScoreResult{}, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp messagesResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return model.ScoreResult{}, fmt.Errorf("decode response: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return model.ScoreResult{}, fmt.Errorf("empty anthropic response")
	}

	return parseScoreResponse(apiResp.Content[0].Text)
}

func offerText(offer model.Offer) string {
	return fmt.Sprintf("Título: %s\nFuente: %s\nLink: %s\nDescripción:\n%s",
		offer.Title, offer.Source, offer.Link, offer.Description)
}

func parseScoreResponse(text string) (model.ScoreResult, error) {
	summaryMatch := summaryRe.FindStringSubmatch(text)
	scoreMatch := scoreRe.FindStringSubmatch(text)
	if len(summaryMatch) < 2 || len(scoreMatch) < 2 {
		return model.ScoreResult{}, fmt.Errorf("unable to parse scoring response: %s", strings.TrimSpace(text))
	}

	score, err := parseScore(scoreMatch[1])
	if err != nil {
		return model.ScoreResult{}, err
	}

	return model.ScoreResult{
		Summary: strings.TrimSpace(summaryMatch[1]),
		Score:   score,
	}, nil
}

func parseScore(raw string) (int, error) {
	var score int
	_, err := fmt.Sscanf(raw, "%d", &score)
	if err != nil {
		return 0, fmt.Errorf("invalid score: %w", err)
	}
	if score < 1 || score > 100 {
		return 0, fmt.Errorf("score out of range: %d", score)
	}
	return score, nil
}
