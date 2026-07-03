// Package localconfig manages ~/.ecsctl/config.yaml — the local file that
// points ecsctl at the correct S3 state bucket per named context.
package localconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFileName = ".ecsctl/config.yaml"

// Context holds the remote state configuration for one named context.
type Context struct {
	Bucket   string `yaml:"bucket"`
	Region   string `yaml:"region"`
	Key      string `yaml:"key"` // S3 key prefix, e.g. "prod" → prod/state.json
	Profile  string `yaml:"profile,omitempty"`
	KmsKeyID string `yaml:"kmsKeyId,omitempty"`
}

// Config is the top-level local config file (~/.ecsctl/config.yaml).
type Config struct {
	CurrentContext string             `yaml:"currentContext"`
	Contexts       map[string]Context `yaml:"contexts"`
}

// configPath returns the absolute path to ~/.ecsctl/config.yaml.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, configFileName), nil
}

// Load reads the local config file. Returns an empty Config if none exists yet.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Contexts: map[string]Context{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = map[string]Context{}
	}
	return &cfg, nil
}

// Save writes the config back to ~/.ecsctl/config.yaml, creating the
// directory if it doesn't exist.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// AddContext adds or replaces a named context and optionally sets it as current.
func (cfg *Config) AddContext(name string, ctx Context, setCurrent bool) {
	cfg.Contexts[name] = ctx
	if setCurrent {
		cfg.CurrentContext = name
	}
}

// GetActiveContext returns the Context for the given name, falling back to
// CurrentContext if name is empty. Returns an error if no context is configured.
func (cfg *Config) GetActiveContext(name string) (string, Context, error) {
	if name == "" {
		name = cfg.CurrentContext
	}
	if name == "" {
		return "", Context{}, fmt.Errorf("no context set — run 'ecsctl state init' first")
	}
	ctx, ok := cfg.Contexts[name]
	if !ok {
		return "", Context{}, fmt.Errorf("context %q not found — run 'ecsctl state init --context %s'", name, name)
	}
	return name, ctx, nil
}
