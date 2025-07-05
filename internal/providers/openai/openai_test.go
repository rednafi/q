package openai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"q/internal/config"
)

// fakeClient is an HTTPClient stub for testing.
type fakeClient struct {
	resp *http.Response
}

func (f *fakeClient) Do(req *http.Request) (*http.Response, error) {
	return f.resp, nil
}

func TestPrompt_NoAPIKey(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	p := NewProvider()
	_, err := p.Prompt(context.Background(), "gpt-4", "hi")
	if err == nil || !strings.Contains(err.Error(), "no API key set for openai") {
		t.Errorf("expected no API key error, got %v", err)
	}
}

func TestPrompt_Success(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	data := `{"choices":[{"message":{"content":"world"}}]}`
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(data)),
		}}
	})
	got, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if got != "world" {
		t.Errorf("Prompt = %q; want %q", got, "world")
	}
}

func TestStream_NoAPIKey(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	p := NewProvider()
	_, err := p.Stream(context.Background(), "gpt-4", "hi")
	if err == nil || !strings.Contains(err.Error(), "no API key set for openai") {
		t.Errorf("expected no API key error, got %v", err)
	}
}

func TestStream_Success(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	s := "data: {\"choices\":[{\"delta\":{\"content\":\"h\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"i\"}}]}\n" +
		"data: [DONE]\n"
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(s)),
		}}
	})
	// capture stdout via pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	got, err := p.Stream(context.Background(), "gpt-4", "prompt")
	if err != nil {
		w.Close()
		os.Stdout = old
		t.Fatalf("Stream error: %v", err)
	}
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy error: %v", err)
	}
	if buf.String() != "hi" {
		t.Errorf("Stream output = %q; want %q", buf.String(), "hi")
	}
	if got != "hi" {
		t.Errorf("Stream return = %q; want %q", got, "hi")
	}
}

func TestNameAndSupportedModels(t *testing.T) {
	p := NewProvider()
	if got := p.Name(); got != "openai" {
		t.Errorf("Name() = %q; want %q", got, "openai")
	}
	models := p.SupportedModels()
	if len(models) == 0 {
		t.Errorf("SupportedModels() = %v; want non-empty slice", models)
	}
}

// fakeClientErr is an HTTPClient stub that returns an error.
type fakeClientErr struct{}

func (f *fakeClientErr) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("fail")
}

func TestPrompt_HTTPError(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClientErr{}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Errorf("expected HTTP error, got %v", err)
	}
}

func TestPrompt_NoResponse(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return empty choices
	data := `{"choices":[]}`
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(data)),
		}}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected no response error, got %v", err)
	}
}

func TestPrompt_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return invalid JSON
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("invalid")),
		}}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil {
		t.Error("expected JSON unmarshal error, got nil")
	}
}

func TestPrompt_EmptyContent(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return choice with empty content
	data := `{"choices":[{"message":{"content":""}}]}`
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(data)),
		}}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "empty response") {
		t.Errorf("expected no content error, got %v", err)
	}
}

func TestPrompt_HTTPStatusError(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return error status
	body := `{"error":{"message":"API key invalid"}}`
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{
			resp: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "Invalid API key for openai") {
		t.Errorf("expected invalid API key error, got %v", err)
	}
}

func TestPrompt_GenericHTTPStatusError(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return error status with non-API key related error
	body := `{"error":{"message":"Rate limit exceeded"}}`
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{
			resp: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "API error: Rate limit exceeded") {
		t.Errorf("expected API error message, got %v", err)
	}
}

func TestPrompt_HTTPStatusErrorInvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return error status with invalid JSON body
	body := `invalid json response`
	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{
			resp: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewBufferString(body)),
			},
		}
	})
	_, err := p.Prompt(context.Background(), "gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "API request failed with status 500") {
		t.Errorf("expected generic HTTP status error, got %v", err)
	}
}

func TestChatPrompt_ConversationHistory(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	// First response
	data1 := `{"choices":[{"message":{"content":"Hello! How can I help you today?"}}]}`
	// Second response that should reference the conversation
	data2 := `{"choices":[{"message":{"content":"Yes, I remember you asked about the weather. It's sunny today!"}}]}`

	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(data1)),
		}}
	})

	// First message
	got1, err := p.ChatPrompt(context.Background(), "gpt-4", "Hello")
	if err != nil {
		t.Fatalf("ChatPrompt error: %v", err)
	}
	if got1 != "Hello! How can I help you today?" {
		t.Errorf("ChatPrompt = %q; want %q", got1, "Hello! How can I help you today?")
	}

	// Update the fake client to return the second response
	p.client = &fakeClient{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(data2)),
	}}

	// Second message - should include conversation history
	got2, err := p.ChatPrompt(context.Background(), "gpt-4", "What's the weather like?")
	if err != nil {
		t.Fatalf("ChatPrompt error: %v", err)
	}
	if got2 != "Yes, I remember you asked about the weather. It's sunny today!" {
		t.Errorf("ChatPrompt = %q; want %q", got2, "Yes, I remember you asked about the weather. It's sunny today!")
	}
}

func TestChatStream_ConversationHistory(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	s := "data: {\"choices\":[{\"delta\":{\"content\":\"H\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"i\"}}]}\n" +
		"data: [DONE]\n"

	p := NewProvider(func(p *Provider) {
		p.client = &fakeClient{resp: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(s)),
		}}
	})

	// capture stdout via pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	old := os.Stdout
	os.Stdout = w

	got, err := p.ChatStream(context.Background(), "gpt-4", "Hello")
	if err != nil {
		w.Close()
		os.Stdout = old
		t.Fatalf("ChatStream error: %v", err)
	}

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy error: %v", err)
	}
	if buf.String() != "Hi" {
		t.Errorf("ChatStream output = %q; want %q", buf.String(), "Hi")
	}
	if got != "Hi" {
		t.Errorf("ChatStream return = %q; want %q", got, "Hi")
	}
}

func TestResetChat(t *testing.T) {
	p := NewProvider()

	// Add some conversation history
	p.push("user", "Hello")
	p.push("assistant", "Hi there!")

	if len(p.history) != 2 {
		t.Errorf("Expected 2 messages in history, got %d", len(p.history))
	}

	p.ResetChat()

	if len(p.history) != 0 {
		t.Errorf("Expected 0 messages in history after reset, got %d", len(p.history))
	}
}
