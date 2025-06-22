package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"q/internal/config"
	"q/internal/httpclient"
	"q/internal/providers"
)

const (
	defaultAPIURL = "https://api.openai.com/v1/chat/completions"
	errKeyFmt     = "no API key set for %s; use 'q keys set --provider %[1]s --key KEY'"
	ssePrefix     = "data: "
)

// APIError represents an error response from the OpenAI API
type APIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// parseAPIError attempts to parse an API error response and return a helpful message
func parseAPIError(providerName string, statusCode int, body []byte) error {
	// Try to parse as API error
	var apiErr APIError
	if err := json.Unmarshal(body, &apiErr); err == nil {
		// Check for invalid API key error
		if statusCode == http.StatusUnauthorized ||
			strings.Contains(apiErr.Error.Code, "invalid_api_key") ||
			strings.Contains(apiErr.Error.Message, "Incorrect API key") {
			return &providers.InvalidAPIKeyError{Provider: providerName}
		}
		// Return the original API error message for other cases
		return fmt.Errorf("API error: %s", apiErr.Error.Message)
	}

	// Fallback to generic error
	return fmt.Errorf("API request failed with status %d: %s", statusCode, string(body))
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type Provider struct {
	client httpclient.HTTPClient
	apiURL string

	mu   sync.Mutex
	hist []Message
}

func NewProvider(opts ...func(*Provider)) *Provider {
	p := &Provider{client: http.DefaultClient, apiURL: defaultAPIURL}
	for _, o := range opts {
		o(p)
	}
	return p
}

// ------------------------------------------------------------------
// Public API
// ------------------------------------------------------------------

func (p *Provider) Name() string { return "openai" }

func (p *Provider) SupportedModels() []string {
	return []string{
		"gpt-3.5-turbo", "gpt-3.5-turbo-0613",
		"gpt-4o", "gpt-4o-mini",
		"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
		"o3-mini", "o3", "o3-pro",
		"o4-mini",
	}
}

// Prompt: one-shot, non-streaming.
func (p *Provider) Prompt(ctx context.Context, model, prompt string) (string, error) {
	return p.send(
		ctx, model, []Message{{Role: "user", Content: prompt}}, false, nil,
	)
}

// Stream: one-shot, streaming; returns the full reply too.
func (p *Provider) Stream(ctx context.Context, model, prompt string) (string, error) {
	var buf strings.Builder
	_, err := p.send(
		ctx, model, []Message{{Role: "user", Content: prompt}}, true,
		func(s string) {
			fmt.Print(s)
			buf.WriteString(s)
		},
	)
	return buf.String(), err
}

// ChatPrompt: keeps conversation history.
func (p *Provider) ChatPrompt(ctx context.Context, model, msg string) (string, error) {
	p.appendHistory("user", msg)
	out, err := p.send(ctx, model, p.historyCopy(), false, nil)
	if err == nil {
		p.appendHistory("assistant", out)
	}
	return out, err
}

// ChatStream: streaming with history; returns the collected reply.
func (p *Provider) ChatStream(ctx context.Context, model, msg string) (string, error) {
	p.appendHistory("user", msg)

	var buf strings.Builder
	_, err := p.send(ctx, model, p.historyCopy(), true, func(s string) {
		fmt.Print(s)
		buf.WriteString(s)
	})
	if err == nil && buf.Len() > 0 {
		p.appendHistory("assistant", buf.String())
	}
	return buf.String(), err
}

// ResetChat clears history.
func (p *Provider) ResetChat() { p.mu.Lock(); p.hist = nil; p.mu.Unlock() }

// ------------------------------------------------------------------
// Internals
// ------------------------------------------------------------------

func (p *Provider) send(
	ctx context.Context,
	model string,
	msgs []Message,
	stream bool,
	onDelta func(string),
) (string, error) {
	key, err := config.GetAPIKey(p.Name())
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", fmt.Errorf(errKeyFmt, p.Name())
	}

	body, _ := json.Marshal(chatRequest{Model: model, Messages: msgs, Stream: stream})
	req, _ := http.NewRequestWithContext(
		ctx, http.MethodPost, p.apiURL, bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", parseAPIError(p.Name(), resp.StatusCode, body)
	}

	// ---------------- Non-streaming ----------------
	if !stream {
		var res chatResponse
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return "", err
		}
		if len(res.Choices) == 0 || res.Choices[0].Message.Content == "" {
			return "", errors.New("openai: empty response")
		}
		return res.Choices[0].Message.Content, nil
	}

	// ---------------- Streaming ----------------
	scanner := bufio.NewScanner(resp.Body)
	var full strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, ssePrefix) {
			continue
		}
		data := strings.TrimPrefix(line, ssePrefix)
		if data == "[DONE]" {
			break
		}
		var chunk chatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		part := chunk.Choices[0].Delta.Content
		if onDelta != nil {
			onDelta(part)
		}
		full.WriteString(part)
	}
	return full.String(), scanner.Err()
}

func (p *Provider) appendHistory(role, content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hist = append(p.hist, Message{Role: role, Content: content})
}

func (p *Provider) historyCopy() []Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Message(nil), p.hist...) // defensive copy
}
