package providers

import (
	"slices"
	"sync"
)

// Provider is implemented by all vendor backends (e.g. OpenAI, Google).
type Provider interface {
	// Name returns the vendor identifier (e.g., "openai", "google").
	Name() string
	// SupportedModels returns the list of model identifiers for this provider
	// (e.g., {"gpt-4", "gpt-4o"}).
	SupportedModels() []string
	// Prompt sends a one-shot prompt to the specified model.
	Prompt(model, prompt string) (string, error)
	// Chat starts an interactive REPL session with the specified model.
	Chat(model string) error
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
