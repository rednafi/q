package providers

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// Provider is implemented by all vendor backends (e.g. OpenAI).
type Provider interface {
	// Name returns the vendor identifier (e.g., "openai").
	Name() string

	// SupportedModels returns the list of model identifiers for this provider
	// (e.g., {"gpt-4", "gpt-4o"}).
	SupportedModels() []string

	// Prompt sends a one-shot prompt to the specified model.
	Prompt(ctx context.Context, model, prompt string) (string, error)

	// Stream sends a one-shot prompt and streams the response as tokens.
	// Returns the full response and any error.
	Stream(ctx context.Context, model, prompt string) (string, error)

	// ChatPrompt sends a message in a conversation context and returns the full response.
	// It maintains conversation history internally.
	ChatPrompt(ctx context.Context, model, message string) (string, error)

	// ChatStream sends a message in a conversation context and streams the response.
	// It maintains conversation history internally.
	// Returns the full response and any error.
	ChatStream(ctx context.Context, model, message string) (string, error)

	// ResetChat clears the conversation history for the provider.
	ResetChat()
}

// Registry stores and manages named providers.
type Registry struct {
	mu   sync.RWMutex
	data map[string]Provider
}

// NewRegistry returns a new, empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		data: make(map[string]Provider),
	}
}

// Register adds one or more providers. It panics if any name is duplicated.
func (r *Registry) Register(ps ...Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range ps {
		name := p.Name()
		if _, exists := r.data[name]; exists {
			panic("provider already registered: " + name)
		}
		r.data[name] = p
	}
}

// Lookup returns the provider with the given name, if found.
func (r *Registry) Lookup(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.data[name]
	return p, ok
}

// Names returns a sorted list of all registered provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.data))
	for name := range r.data {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// InvalidAPIKeyError represents an invalid API key error that any provider can return
type InvalidAPIKeyError struct {
	Provider string
}

func (e *InvalidAPIKeyError) Error() string {
	return fmt.Sprintf(
		"Invalid API key for %s. Set your key with:\n  "+
			"q keys set --provider %s --key YOUR_API_KEY",
		e.Provider,
		e.Provider,
	)
}

// IsInvalidAPIKeyError checks if an error is an InvalidAPIKeyError
func IsInvalidAPIKeyError(err error) bool {
	_, ok := err.(*InvalidAPIKeyError)
	return ok
}
