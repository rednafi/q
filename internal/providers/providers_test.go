package providers

import (
   "reflect"
   "testing"
)

// dummyProvider is a minimal Provider implementation for testing.
type dummyProvider struct {
   name string
}

func (d *dummyProvider) Name() string                 { return d.name }
func (d *dummyProvider) SupportedModels() []string    { return []string{} }
func (d *dummyProvider) Prompt(model, prompt string) (string, error) { return "", nil }
func (d *dummyProvider) Chat(model string) error      { return nil }

func TestRegisterGetProviders(t *testing.T) {
   // reset registry
   orig := registry
   defer func() { registry = orig }()
   registry = make(map[string]Provider)

   p := &dummyProvider{name: "test"}
   Register(p)
   got, ok := Get("test")
   if !ok || got != p {
       t.Errorf("Get(\"test\") = %v, %v; want %v, true", got, ok, p)
   }
   ps := Providers()
   if len(ps) != 1 || ps[0] != "test" {
       t.Errorf("Providers() = %v; want [\"test\"]", ps)
   }
}

func TestRegisterDuplicatePanics(t *testing.T) {
   orig := registry
   defer func() { registry = orig }()
   registry = make(map[string]Provider)

   p := &dummyProvider{name: "dup"}
   Register(p)
   defer func() {
       if r := recover(); r == nil {
           t.Errorf("expected panic on duplicate Register")
       }
   }()
   Register(p)
}

func TestProvidersSorted(t *testing.T) {
   orig := registry
   defer func() { registry = orig }()
   registry = make(map[string]Provider)
   registry["b"] = &dummyProvider{name: "b"}
   registry["a"] = &dummyProvider{name: "a"}
   want := []string{"a", "b"}
   if got := Providers(); !reflect.DeepEqual(got, want) {
       t.Errorf("Providers() = %v; want %v", got, want)
   }
}