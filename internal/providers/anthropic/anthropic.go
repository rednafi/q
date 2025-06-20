package anthropic

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

// Provider implements the anthropic provider for Claude models.
// It holds an HTTP client for making requests, enabling dependency injection.
type Provider struct {
	client httpclient.HTTPClient
}

// New returns a new Anthropic Provider using the default HTTP client.
func New() *Provider {
	return &Provider{client: http.DefaultClient}
}

// NewWithClient returns a new Anthropic Provider with the provided HTTP client.
func NewWithClient(c httpclient.HTTPClient) *Provider {
	return &Provider{client: c}
}

// Name returns the vendor name.
func (p *Provider) Name() string { return "anthropic" }

// SupportedModels lists the Anthropic Claude model identifiers supported by q.
func (p *Provider) SupportedModels() []string {
	return []string{
		"claude-opus-4-20250514",
		"claude-sonnet-4-20250514",
		"claude-3.7-sonnet-20250219",
		"claude-3.5-haiku-20241022",
	}
}

// Prompt sends a one-shot prompt to the Anthropic Messages API.
func (p *Provider) Prompt(model, prompt string) (string, error) {
	key, err := config.GetAPIKey(p.Name())
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", fmt.Errorf("no API key set for %s; use 'q set key --provider %s --key KEY'", p.Name(), p.Name())
	}
	// Anthropic API expects input as a list of messages. Single prompt is as user.
	apiURL := "https://api.anthropic.com/v1/messages"
	body := map[string]any{
		"model":      model,
		"max_tokens": 1024,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
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

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respData, &res); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if len(res.Content) == 0 {
		return "", fmt.Errorf("no response from anthropic")
	}
	if res.Content[0].Text == "" {
		return "", fmt.Errorf("no content in response from anthropic")
	}
	return res.Content[0].Text, nil
}

// Chat starts an interactive REPL with the specified Claude model.
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
