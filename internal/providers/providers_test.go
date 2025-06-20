package providers_test

import (
	"reflect"
	"testing"

	"q/internal/providers"
)

// dummyProvider is a minimal Provider implementation for testing.
type dummyProvider struct {
	name string
}

func (d *dummyProvider) Name() string                                { return d.name }
func (d *dummyProvider) SupportedModels() []string                   { return []string{} }
func (d *dummyProvider) Prompt(model, prompt string) (string, error) { return "", nil }
func (d *dummyProvider) Chat(model string) error                     { return nil }

func TestRegistryStruct(t *testing.T) {
	// Test Registry struct directly
	reg := providers.NewRegistry()
	p := &dummyProvider{name: "test"}
	reg.Register(p)

	got, ok := reg.Lookup("test")
	if !ok || got != p {
		t.Errorf("reg.Lookup(\"test\") = %v, %v; want %v, true", got, ok, p)
	}

	ps := reg.Names()
	if len(ps) != 1 || ps[0] != "test" {
		t.Errorf("reg.Names() = %v; want [\"test\"]", ps)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	reg := providers.NewRegistry()
	p := &dummyProvider{name: "dup"}
	reg.Register(p)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate Register")
		}
	}()
	reg.Register(p)
}

func TestProvidersSorted(t *testing.T) {
	reg := providers.NewRegistry()
	// Register providers in reverse order to test sorting
	reg.Register(&dummyProvider{name: "b"})
	reg.Register(&dummyProvider{name: "a"})
	want := []string{"a", "b"}
	if got := reg.Names(); !reflect.DeepEqual(got, want) {
		t.Errorf("reg.Names() = %v; want %v", got, want)
	}
}

func TestMultipleProvidersRegistration(t *testing.T) {
	reg := providers.NewRegistry()
	p1 := &dummyProvider{name: "provider1"}
	p2 := &dummyProvider{name: "provider2"}

	reg.Register(p1, p2)

	got1, ok1 := reg.Lookup("provider1")
	if !ok1 || got1 != p1 {
		t.Errorf("reg.Lookup(\"provider1\") = %v, %v; want %v, true", got1, ok1, p1)
	}

	got2, ok2 := reg.Lookup("provider2")
	if !ok2 || got2 != p2 {
		t.Errorf("reg.Lookup(\"provider2\") = %v, %v; want %v, true", got2, ok2, p2)
	}
}
