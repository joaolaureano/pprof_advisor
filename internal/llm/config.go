package llm

import (
	"fmt"
	"os"
	"sort"
	"sync"
)

// Config holds provider-agnostic LLM client configuration.
//
// The zero value is usable: New will fill empty fields from environment
// variables, so most callers can pass an empty Config{} and let env vars
// take effect.
type Config struct {
	// Provider is the provider name, one of the registered providers.
	// Default: PROFADVISOR_PROVIDER env var, else DefaultProvider.
	Provider string

	// Model is the model identifier to request from the provider.
	// Default: PROFADVISOR_MODEL env var, else the provider's default.
	Model string

	// BaseURL is the API endpoint base URL, for a gateway, a proxy, or a
	// local server speaking a provider's wire format.
	// Default: PROFADVISOR_BASE_URL env var, else the provider's default.
	BaseURL string

	// APIKey is the authentication token for the provider.
	// Default: PROFADVISOR_API_KEY env var, else the provider's conventional
	// environment variable, which only that provider knows the name of.
	APIKey string
}

// New returns a Client for the configured provider, filling in defaults from
// environment variables. It returns an error naming every registered provider
// if the Provider name is not recognized.
func New(cfg Config) (Client, error) {
	// Fill in defaults from environment.
	if cfg.Provider == "" {
		cfg.Provider = os.Getenv("PROFADVISOR_PROVIDER")
	}
	if cfg.Provider == "" {
		cfg.Provider = DefaultProvider
	}
	if cfg.Provider == "" {
		// Only reachable if nothing was linked in; say so plainly rather than
		// reporting an unknown provider named "".
		return nil, fmt.Errorf("llm: no provider selected and none registered; " +
			"import internal/llm/providers")
	}
	if cfg.Model == "" {
		cfg.Model = os.Getenv("PROFADVISOR_MODEL")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = os.Getenv("PROFADVISOR_BASE_URL")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("PROFADVISOR_API_KEY")
	}

	// Look up the factory.
	registryMu.RLock()
	factory, ok := registry[cfg.Provider]
	registryMu.RUnlock()

	if !ok {
		providers := Providers()
		return nil, fmt.Errorf("llm: unknown provider %q; registered providers: %v", cfg.Provider, providers)
	}

	// Let the factory handle missing API key, since it knows the vendor's
	// conventional env var and can produce a more helpful error message.
	return factory(cfg)
}

// DefaultProvider is the provider used when neither the caller nor
// PROFADVISOR_PROVIDER names one. It is set by whatever package registers the
// providers, not hardcoded here: a default has to name somebody, and this file
// is the vendor-neutral core.
var DefaultProvider string

// Factory is a provider's constructor function.
type Factory func(cfg Config) (Client, error)

var (
	registry   = make(map[string]Factory)
	registryMu sync.RWMutex
)

// Register adds a provider factory to the registry. It panics if the name
// is already registered.
func Register(name string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[name]; ok {
		panic(fmt.Sprintf("llm: provider %q already registered", name))
	}
	registry[name] = f
}

// Providers returns a sorted list of registered provider names.
func Providers() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
