package config

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}
	return f.Name()
}

// --- LoadClusterConfig ---

func TestLoadClusterConfig_Valid(t *testing.T) {
	yaml := `
name: my-cluster
region: eu-west-2
capacityProviders:
  - FARGATE
  - FARGATE_SPOT
tags:
  env: production
  team: platform
`
	path := writeTemp(t, yaml)
	cfg, err := LoadClusterConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "my-cluster" {
		t.Errorf("Name: got %q, want %q", cfg.Name, "my-cluster")
	}
	if cfg.Region != "eu-west-2" {
		t.Errorf("Region: got %q, want %q", cfg.Region, "eu-west-2")
	}
	if len(cfg.CapacityProviders) != 2 {
		t.Errorf("CapacityProviders: got %d, want 2", len(cfg.CapacityProviders))
	}
	if cfg.Tags["env"] != "production" {
		t.Errorf("Tags[env]: got %q, want %q", cfg.Tags["env"], "production")
	}
}

func TestLoadClusterConfig_MissingName(t *testing.T) {
	path := writeTemp(t, "region: eu-west-2\n")
	_, err := LoadClusterConfig(path)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoadClusterConfig_FileNotFound(t *testing.T) {
	_, err := LoadClusterConfig(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadClusterConfig_InvalidYAML(t *testing.T) {
	path := writeTemp(t, "name: [unclosed")
	_, err := LoadClusterConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

// --- LoadServiceConfig ---

func TestLoadServiceConfig_Valid(t *testing.T) {
	yaml := `
name: my-service
cluster: my-cluster
taskDefinition: my-task:3
launchType: FARGATE
desiredCount: 2
network:
  subnets:
    - subnet-abc123
  securityGroups:
    - sg-abc123
  assignPublicIp: ENABLED
`
	path := writeTemp(t, yaml)
	cfg, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "my-service" {
		t.Errorf("Name: got %q, want %q", cfg.Name, "my-service")
	}
	if cfg.Cluster != "my-cluster" {
		t.Errorf("Cluster: got %q, want %q", cfg.Cluster, "my-cluster")
	}
	if cfg.TaskDefinition != "my-task:3" {
		t.Errorf("TaskDefinition: got %q, want %q", cfg.TaskDefinition, "my-task:3")
	}
	if cfg.DesiredCount != 2 {
		t.Errorf("DesiredCount: got %d, want 2", cfg.DesiredCount)
	}
	if cfg.NetworkConfig == nil {
		t.Fatal("NetworkConfig should not be nil")
	}
	if len(cfg.NetworkConfig.Subnets) != 1 || cfg.NetworkConfig.Subnets[0] != "subnet-abc123" {
		t.Errorf("Subnets: got %v", cfg.NetworkConfig.Subnets)
	}
}

func TestLoadServiceConfig_Defaults(t *testing.T) {
	yaml := `
name: my-service
cluster: my-cluster
taskDefinition: my-task:1
`
	path := writeTemp(t, yaml)
	cfg, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// LaunchType should default to FARGATE
	if cfg.LaunchType != "FARGATE" {
		t.Errorf("LaunchType default: got %q, want %q", cfg.LaunchType, "FARGATE")
	}
	// DesiredCount should default to 1
	if cfg.DesiredCount != 1 {
		t.Errorf("DesiredCount default: got %d, want 1", cfg.DesiredCount)
	}
}

func TestLoadServiceConfig_MissingName(t *testing.T) {
	yaml := `
cluster: my-cluster
taskDefinition: my-task:1
`
	path := writeTemp(t, yaml)
	_, err := LoadServiceConfig(path)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestLoadServiceConfig_MissingCluster(t *testing.T) {
	yaml := `
name: my-service
taskDefinition: my-task:1
`
	path := writeTemp(t, yaml)
	_, err := LoadServiceConfig(path)
	if err == nil {
		t.Fatal("expected error for missing cluster, got nil")
	}
}

func TestLoadServiceConfig_MissingTaskDefinition(t *testing.T) {
	yaml := `
name: my-service
cluster: my-cluster
`
	path := writeTemp(t, yaml)
	_, err := LoadServiceConfig(path)
	if err == nil {
		t.Fatal("expected error for missing taskDefinition, got nil")
	}
}
