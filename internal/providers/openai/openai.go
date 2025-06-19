package openai

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

// Provider implements the openai provider.
type Provider struct{}

// New returns a new OpenAI Provider.
func New() *Provider { return &Provider{} }

// Name returns the vendor name.
func (p *Provider) Name() string { return "openai" }

// SupportedModels lists the OpenAI model identifiers.
func (p *Provider) SupportedModels() []string {
	return []string{"gpt-4", "gpt-4o", "o3", "gpt-4.1"}
}

// Prompt sends a one-shot prompt to the OpenAI Chat Completions API.
func (p *Provider) Prompt(model, prompt string) (string, error) {
	key, err := config.GetAPIKey(p.Name())
	if err != nil {
		return "", err
	}
	if key == "" {
		return "", fmt.Errorf("no API key set for %s; use 'q set key --provider %s --key KEY'", p.Name(), p.Name())
	}
	body := map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpclient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respData, &res); err != nil {
		return "", err
	}
	if len(res.Choices) == 0 {
		return "", fmt.Errorf("no response from openai")
	}
	return res.Choices[0].Message.Content, nil
}

// Stream sends a one-shot prompt and streams the response as tokens.
func (p *Provider) Stream(model, prompt string) error {
	key, err := config.GetAPIKey(p.Name())
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("no API key set for %s; use 'q keys set --provider %s --key KEY'", p.Name(), p.Name())
	}
	body := map[string]any{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": prompt}},
		"stream":   true,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpclient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if len(line) < 6 || !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		chunkData := line[6:]
		if bytes.Equal(bytes.TrimSpace(chunkData), []byte("[DONE]")) {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(chunkData, &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}
	return nil
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
