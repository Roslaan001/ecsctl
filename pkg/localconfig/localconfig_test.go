package localconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempHome sets HOME to a temp dir for the duration of the test,
// so config reads/writes don't touch the real ~/.ecsctl/config.yaml.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestLoad_NoFileReturnsEmpty(t *testing.T) {
	withTempHome(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("expected empty CurrentContext, got %q", cfg.CurrentContext)
	}
	if len(cfg.Contexts) != 0 {
		t.Errorf("expected empty Contexts, got %v", cfg.Contexts)
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	withTempHome(t)

	cfg := &Config{
		CurrentContext: "prod",
		Contexts: map[string]Context{
			"prod": {Bucket: "my-bucket", Region: "eu-west-2", Key: "prod"},
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.CurrentContext != "prod" {
		t.Errorf("CurrentContext: got %q, want %q", loaded.CurrentContext, "prod")
	}
	ctx, ok := loaded.Contexts["prod"]
	if !ok {
		t.Fatal("context 'prod' not found after load")
	}
	if ctx.Bucket != "my-bucket" {
		t.Errorf("Bucket: got %q, want %q", ctx.Bucket, "my-bucket")
	}
	if ctx.Key != "prod" {
		t.Errorf("Key: got %q, want %q", ctx.Key, "prod")
	}
}

func TestSave_CreatesDirectoryIfMissing(t *testing.T) {
	home := withTempHome(t)

	cfg := &Config{CurrentContext: "test", Contexts: map[string]Context{}}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	expectedPath := filepath.Join(home, configFileName)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected config file at %s, not found", expectedPath)
	}
}

// --- GetActiveContext ---

func TestGetActiveContext_NoContextSet(t *testing.T) {
	cfg := &Config{Contexts: map[string]Context{}}

	_, _, err := cfg.GetActiveContext("")
	if err == nil {
		t.Fatal("expected error when no context is set, got nil")
	}
}

func TestGetActiveContext_UnknownContext(t *testing.T) {
	cfg := &Config{
		CurrentContext: "prod",
		Contexts: map[string]Context{
			"prod": {Bucket: "b", Region: "eu-west-2", Key: "prod"},
		},
	}

	_, _, err := cfg.GetActiveContext("staging")
	if err == nil {
		t.Fatal("expected error for unknown context 'staging', got nil")
	}
}

func TestGetActiveContext_FallsBackToCurrentContext(t *testing.T) {
	cfg := &Config{
		CurrentContext: "prod",
		Contexts: map[string]Context{
			"prod": {Bucket: "prod-bucket", Region: "eu-west-2", Key: "prod"},
		},
	}

	name, ctx, err := cfg.GetActiveContext("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "prod" {
		t.Errorf("name: got %q, want %q", name, "prod")
	}
	if ctx.Bucket != "prod-bucket" {
		t.Errorf("Bucket: got %q, want %q", ctx.Bucket, "prod-bucket")
	}
}

func TestGetActiveContext_ExplicitNameOverridesDefault(t *testing.T) {
	cfg := &Config{
		CurrentContext: "prod",
		Contexts: map[string]Context{
			"prod":    {Bucket: "prod-bucket", Region: "eu-west-2", Key: "prod"},
			"staging": {Bucket: "staging-bucket", Region: "eu-west-1", Key: "staging"},
		},
	}

	name, ctx, err := cfg.GetActiveContext("staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "staging" {
		t.Errorf("name: got %q, want %q", name, "staging")
	}
	if ctx.Bucket != "staging-bucket" {
		t.Errorf("Bucket: got %q, want %q", ctx.Bucket, "staging-bucket")
	}
}

func TestAddContext_SetsCurrent(t *testing.T) {
	cfg := &Config{Contexts: map[string]Context{}}

	cfg.AddContext("prod", Context{Bucket: "b", Region: "eu-west-2", Key: "prod"}, true)

	if cfg.CurrentContext != "prod" {
		t.Errorf("CurrentContext: got %q, want %q", cfg.CurrentContext, "prod")
	}
}

func TestAddContext_DoesNotOverrideCurrent(t *testing.T) {
	cfg := &Config{
		CurrentContext: "prod",
		Contexts:       map[string]Context{},
	}

	cfg.AddContext("staging", Context{Bucket: "b", Region: "eu-west-1", Key: "staging"}, false)

	if cfg.CurrentContext != "prod" {
		t.Errorf("CurrentContext should remain 'prod', got %q", cfg.CurrentContext)
	}
}
