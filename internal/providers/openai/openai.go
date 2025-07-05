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
	ssePrefix     = "data: "
	errKeyFmt     = "no API key set for %s; use 'q keys set --provider %[1]s --key KEY'"
)

var supportedModels = []string{
	"gpt-3.5-turbo", "gpt-3.5-turbo-0613",
	"gpt-4o", "gpt-4o-mini",
	"gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
	"o3-mini", "o3", "o3-pro",
	"o4-mini",
}

type apiErr struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func handleAPIError(provider string, statusCode int, responseBody []byte) error {
	var apiError apiErr
	if json.Unmarshal(responseBody, &apiError) == nil { // parsed
		// bad / missing key?
		if statusCode == http.StatusUnauthorized ||
			strings.Contains(apiError.Error.Code, "invalid_api_key") ||
			strings.Contains(apiError.Error.Message, "Incorrect API key") {
			return &providers.InvalidAPIKeyError{Provider: provider}
		}
		return fmt.Errorf("API error: %s", apiError.Error.Message)
	}
	return fmt.Errorf("API request failed with status %d: %s", statusCode, string(responseBody))
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct{ Content string } `json:"message"`
		Delta   struct{ Content string } `json:"delta"`
	} `json:"choices"`
}

type Provider struct {
	client httpclient.HTTPClient
	apiURL string

	mu      sync.Mutex
	history []Message
}

func NewProvider(opts ...func(*Provider)) *Provider {
	p := &Provider{client: http.DefaultClient, apiURL: defaultAPIURL}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *Provider) Name() string              { return "openai" }
func (p *Provider) SupportedModels() []string { return supportedModels }

func (p *Provider) Prompt(ctx context.Context, model, prompt string) (string, error) {
	return p.send(ctx, model, []Message{{Role: "user", Content: prompt}}, false, nil)
}

func (p *Provider) Stream(ctx context.Context, model, prompt string) (string, error) {
	var out strings.Builder
	_, err := p.send(ctx, model, []Message{{Role: "user", Content: prompt}}, true, func(s string) {
		fmt.Print(s)
		out.WriteString(s)
	})
	return out.String(), err
}

func (p *Provider) ChatPrompt(ctx context.Context, model, msg string) (string, error) {
	p.push("user", msg)
	resp, err := p.send(ctx, model, p.copyHistory(), false, nil)
	if err == nil {
		p.push("assistant", resp)
	}
	return resp, err
}

func (p *Provider) ChatStream(ctx context.Context, model, msg string) (string, error) {
	p.push("user", msg)

	var out strings.Builder
	_, err := p.send(ctx, model, p.copyHistory(), true, func(s string) {
		fmt.Print(s)
		out.WriteString(s)
	})
	if err == nil && out.Len() > 0 {
		p.push("assistant", out.String())
	}
	return out.String(), err
}

func (p *Provider) ResetChat() { p.mu.Lock(); p.history = nil; p.mu.Unlock() }

func (p *Provider) send(
	ctx context.Context,
	model string,
	msgs []Message,
	stream bool,
	onDelta func(string),
) (string, error) {

	key, err := config.GetAPIKey(p.Name())
	switch {
	case err != nil:
		return "", err
	case key == "":
		return "", fmt.Errorf(errKeyFmt, p.Name())
	}

	body, _ := json.Marshal(chatReq{Model: model, Messages: msgs, Stream: stream})

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", handleAPIError(p.Name(), resp.StatusCode, responseBody)
	}

	/* -------- Non-streaming -------- */
	if !stream {
		var response chatResp
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return "", err
		}
		if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
			return "", errors.New("openai: empty response")
		}
		return response.Choices[0].Message.Content, nil
	}

	/* -------- Streaming -------- */
	scanner := bufio.NewScanner(resp.Body)
	var fullResponse strings.Builder

	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return fullResponse.String(), ctx.Err()
		default:
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, ssePrefix) {
			continue
		}
		data := strings.TrimPrefix(line, ssePrefix)
		if data == "[DONE]" {
			break
		}
		var chunk chatResp
		if json.Unmarshal([]byte(data), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		content := chunk.Choices[0].Delta.Content
		if onDelta != nil {
			onDelta(content)
		}
		fullResponse.WriteString(content)
	}
	return fullResponse.String(), scanner.Err()
}

func (p *Provider) push(role, content string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = append(p.history, Message{Role: role, Content: content})
}

func (p *Provider) copyHistory() []Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Message(nil), p.history...) // defensive copy
}
