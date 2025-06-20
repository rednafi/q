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
		return "", fmt.Errorf("no API key set for %s; use 'q set key --provider %s --key KEY'", p.Name(), p.Name())
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateMessage", model)
	body := map[string]interface{}{"prompt": map[string]interface{}{"text": prompt}}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
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
	var res struct {
		Candidates []struct {
			Content string `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respData, &res); err != nil {
		return "", err
	}
	if len(res.Candidates) == 0 {
		return "", fmt.Errorf("no response from google/gemini")
	}
	return res.Candidates[0].Content, nil
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
