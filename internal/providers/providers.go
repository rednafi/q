package providers

import (
	"slices"
	"sync"
)

// Provider is the interface each vendor plugin must implement.
type Provider interface {
	// Name returns the vendor identifier (e.g., "openai", "google").
	Name() string
	// SupportedModels returns the list of model identifiers for this provider
	// (e.g., {"gpt-4", "gpt-4o"}).
	SupportedModels() []string
	// Prompt sends a one-shot prompt to the specified model.
	Prompt(model string, prompt string) (string, error)
	// Chat starts an interactive REPL session with the specified model.
	Chat(model string) error
}

var registry = make(map[string]Provider)

// Register adds a new Provider. It panics on duplicate names.
func Register(p Provider) {
	name := p.Name()
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()

	if _, ok := registry[name]; ok {
		panic("provider already registered: " + name)
	}
	registry[name] = p
}

// Get returns the Provider registered under the given name.
func Get(name string) (Provider, bool) {
	p, ok := registry[name]
	return p, ok
}

// Providers returns the sorted list of registered provider names.
func Providers() []string {
	ps := make([]string, 0, len(registry))
	for name := range registry {
		ps = append(ps, name)
	}
	slices.Sort(ps)
	return ps
}
