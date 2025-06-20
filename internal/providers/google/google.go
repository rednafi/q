package google

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"q/internal/config"
	"q/internal/httpclient"
)

// Provider implements the Google Gemini provider.
// It holds an HTTP client for making requests, enabling dependency injection.
type Provider struct {
	client httpclient.HTTPClient
}

// New returns a new Google Provider using the default HTTP client.
func New() *Provider {
	return &Provider{client: http.DefaultClient}
}

// NewWithClient returns a new Google Provider with the provided HTTP client.
func NewWithClient(c httpclient.HTTPClient) *Provider {
	return &Provider{client: c}
}

// Name returns the vendor name.
func (p *Provider) Name() string { return "google" }

// SupportedModels lists the Google Gemini model identifiers.
func (p *Provider) SupportedModels() []string {
	return []string{
		"gemini-1.0-pro",
		"gemini-1.0-pro-vision",
		"gemini-1.5-pro",
		"gemini-1.5-flash",
		"gemini-2.0-flash",
		"gemini-2.0-flash-lite",
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
	}
}

// Prompt sends a one-shot prompt to the Google Gemini API.
func (p *Provider) Prompt(model, prompt string) (string, error) {
	key, err := config.GetAPIKey(p.Name())
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", fmt.Errorf("no API key set for %s; use 'q keys set --provider %s --key KEY'", p.Name(), p.Name())
	}

	// Use the correct Gemini API endpoint
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s", model, key)

	// Correct request format for Gemini API
	body := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{
						"text": prompt,
					},
				},
			},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Check for HTTP error status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respData))
	}

	// Correct response format for Gemini API
	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respData, &res); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(res.Candidates) == 0 {
		return "", fmt.Errorf("no response from google/gemini")
	}

	if len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in response from google/gemini")
	}

	return res.Candidates[0].Content.Parts[0].Text, nil
}

// Chat starts an interactive REPL with the specified model.
func (p *Provider) Chat(model string) error {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("you: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		resp, err := p.Prompt(model, text)
		if err != nil {
			return err
		}
		fmt.Printf("model (%s/%s): %s\n", p.Name(), model, resp)
	}
}
