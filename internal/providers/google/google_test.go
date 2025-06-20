package google

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
	_, err := p.Prompt("gemini-1.5-flash", "hello")
	if err == nil || !strings.Contains(err.Error(), "no API key set for google") {
		t.Errorf("expected no API key error, got %v", err)
	}
}

func TestPrompt_Success(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Updated response format to match the corrected Gemini API
	data := `{"candidates":[{"content":{"parts":[{"text":"hi"}]}}]}`
	p := NewWithClient(&fakeClient{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(data)),
	}})
	got, err := p.Prompt("gemini-1.5-flash", "prompt")
	if err != nil {
		t.Fatalf("Prompt error: %v", err)
	}
	if got != "hi" {
		t.Errorf("Prompt = %q; want %q", got, "hi")
	}
}

func TestNameAndSupportedModels(t *testing.T) {
	p := New()
	if got := p.Name(); got != "google" {
		t.Errorf("Name() = %q; want %q", got, "google")
	}
	models := p.SupportedModels()
	want := []string{
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
	if !reflect.DeepEqual(models, want) {
		t.Errorf("SupportedModels() = %v; want %v", models, want)
	}
}

// TestChat demonstrates interactive Chat reads input and prints response.
func TestChat(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Updated response format to match the corrected Gemini API
	body := `{"candidates":[{"content":{"parts":[{"text":"resp"}]}}]}`
	p := NewWithClient(&fakeClient{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}})
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

	if err := p.Chat("gemini-1.5-flash"); err != nil {
		t.Fatalf("Chat error: %v", err)
	}
	wOut.Close()
	var buf bytes.Buffer
	io.Copy(&buf, rOut)
	out := buf.String()
	if !strings.Contains(out, "model (google/gemini-1.5-flash): resp") {
		t.Errorf("Chat output = %q; want to contain model response", out)
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
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	pErr := NewWithClient(&fakeClientErr{})
	_, err := pErr.Prompt("gemini-1.5-flash", "prompt")
	if err == nil || !strings.Contains(err.Error(), "fail") {
		t.Errorf("expected HTTP error, got %v", err)
	}
}

func TestPrompt_NoResponse(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return empty candidates
	body := `{"candidates":[]}`
	pNoResp := NewWithClient(&fakeClient{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}})
	_, err := pNoResp.Prompt("gemini-1.5-flash", "prompt")
	if err == nil || !strings.Contains(err.Error(), "no response from google/gemini") {
		t.Errorf("expected no response error, got %v", err)
	}
}

func TestPrompt_EmptyContent(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return candidate with empty content parts
	body := `{"candidates":[{"content":{"parts":[]}}]}`
	pEmpty := NewWithClient(&fakeClient{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}})
	_, err := pEmpty.Prompt("gemini-1.5-flash", "prompt")
	if err == nil || !strings.Contains(err.Error(), "no content in response from google/gemini") {
		t.Errorf("expected no content error, got %v", err)
	}
}

func TestPrompt_HTTPStatusError(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	// Stub HTTP client to return error status
	body := `{"error":{"message":"API key invalid"}}`
	pStatusErr := NewWithClient(&fakeClient{
		resp: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		},
	})
	_, err := pStatusErr.Prompt("gemini-1.5-flash", "prompt")
	if err == nil || !strings.Contains(err.Error(), "API request failed with status 401") {
		t.Errorf("expected HTTP status error, got %v", err)
	}
}

func TestPrompt_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	if err := config.SetAPIKey("google", "key"); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}
	pInvalid := NewWithClient(&fakeClient{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("notjson")),
	}})
	_, err := pInvalid.Prompt("gemini-1.5-flash", "prompt")
	if err == nil {
		t.Error("expected JSON unmarshal error, got nil")
	}
}
