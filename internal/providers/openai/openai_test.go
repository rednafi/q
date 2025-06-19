package openai

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"q/internal/config"
	"q/internal/httpclient"
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
	p := New()
	_, err := p.Prompt("gpt-4", "hi")
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
	httpclient.SetClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString(data))}})
	defer httpclient.SetClient(http.DefaultClient)
	p := New()
	got, err := p.Prompt("gpt-4", "prompt")
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
	p := New()
	err := p.Stream("gpt-4", "hi")
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
	s :=
		"data: {\"choices\":[{\"delta\":{\"content\":\"h\"}}]}\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"i\"}}]}\n" +
			"data: [DONE]\n"
	httpclient.SetClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(strings.NewReader(s))}})
	defer httpclient.SetClient(http.DefaultClient)
	// capture stdout via pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe error: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	p := New()
	if err := p.Stream("gpt-4", "prompt"); err != nil {
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
}

// TestChat demonstrates interactive Chat reads input and prints response.
func TestChat(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("openai", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client for Prompt
	data := `{"choices":[{"message":{"content":"out"}}]}`
	httpclient.SetClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString(data))}})
	defer httpclient.SetClient(http.DefaultClient)
	// Prepare stdin with a single message and EOF
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	pw.Write([]byte("hello\n"))
	pw.Close()
	oldStdin := os.Stdin
	os.Stdin = pr
	defer func() { os.Stdin = oldStdin }()
	// Capture stdout
	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	p := New()
	if err := p.Chat("gpt-4"); err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	wOut.Close()
	var buf bytes.Buffer
	io.Copy(&buf, rOut)
	out := buf.String()
	if !strings.Contains(out, "model (openai/gpt-4): out") {
		t.Errorf("Chat output = %q; want to contain model response", out)
	}
}

func TestNameAndSupportedModels(t *testing.T) {
	p := New()
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
	httpclient.SetClient(&fakeClientErr{})
	defer httpclient.SetClient(http.DefaultClient)
	_, err := New().Prompt("gpt-4", "prompt")
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
	httpclient.SetClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString(data))}})
	defer httpclient.SetClient(http.DefaultClient)
	_, err := New().Prompt("gpt-4", "prompt")
	if err == nil || !strings.Contains(err.Error(), "no response from openai") {
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
	httpclient.SetClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString("invalid"))}})
	defer httpclient.SetClient(http.DefaultClient)
	_, err := New().Prompt("gpt-4", "prompt")
	if err == nil {
		t.Error("expected JSON unmarshal error, got nil")
	}
}
