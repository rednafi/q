package anthropic

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"q/internal/config"
)

// fakeClient is a stub HTTPClient returning a preset response.
type fakeClient struct {
	resp *http.Response
}

func (f *fakeClient) Do(req *http.Request) (*http.Response, error) {
	return f.resp, nil
}

// fakeClientErr is a stub HTTPClient that always errors.
type fakeClientErr struct{}

func (f *fakeClientErr) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("fail")
}

func TestNameAndSupportedModels(t *testing.T) {
	p := New()
	if got := p.Name(); got != "anthropic" {
		t.Errorf("Name() = %q; want %q", got, "anthropic")
	}
	want := []string{
		"claude-opus-4-20250514",
		"claude-sonnet-4-20250514",
		"claude-3.7-sonnet-20250219",
		"claude-3.5-haiku-20241022",
	}
	if models := p.SupportedModels(); !reflect.DeepEqual(models, want) {
		t.Errorf("SupportedModels() = %v; want %v", models, want)
	}
}

func TestPrompt_NoAPIKey(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	p := New()
	_, err := p.Prompt("claude-2.1", "hi")
	if err == nil || !strings.Contains(err.Error(), "no API key set for anthropic") {
		t.Errorf("expected no API key error, got %v", err)
	}
}

func TestPrompt_Success(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("anthropic", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	body := `{"content":[{"text":"hello"}]}`

	p := NewWithClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString(body))}})
	got, err := p.Prompt("claude-2.1", "prompt")
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if got != "hello" {
		t.Errorf("Prompt = %q; want %q", got, "hello")
	}
}

func TestPrompt_HTTPError(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("anthropic", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	pErr := NewWithClient(&fakeClientErr{})
	_, err := pErr.Prompt("claude-2.1", "prompt")
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Errorf("expected HTTP error, got %v", err)
	}
}

func TestPrompt_NoResponse(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("anthropic", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	body := `{"content":[]}`

	pNoResp := NewWithClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString(body))}})
	_, err := pNoResp.Prompt("claude-2.1", "prompt")
	if err == nil || !strings.Contains(err.Error(), "no response from anthropic") {
		t.Errorf("expected no response error, got %v", err)
	}
}

func TestPrompt_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("anthropic", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	pInvalid := NewWithClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString("invalid"))}})
	_, err := pInvalid.Prompt("claude-2.1", "prompt")
	if err == nil {
		t.Error("expected JSON unmarshal error, got nil")
	}
}

func TestChat(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("anthropic", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	body := `{"content":[{"text":"resp"}]}`
	p := NewWithClient(&fakeClient{resp: &http.Response{Body: io.NopCloser(bytes.NewBufferString(body))}})

	// Prepare stdin with a single message and EOF
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	pw.Write([]byte("input\n"))
	pw.Close()
	oldStdin := os.Stdin
	os.Stdin = pr
	defer func() { os.Stdin = oldStdin }()

	// Capture stdout
	rOut, wOut, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = wOut
	defer func() { os.Stdout = oldStdout }()

	if err := p.Chat("claude-2.1"); err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	wOut.Close()
	var buf bytes.Buffer
	io.Copy(&buf, rOut)
	out := buf.String()
	if !strings.Contains(out, "model (anthropic/claude-2.1): resp") {
		t.Errorf("Chat output = %q; want to contain model response", out)
	}
}
