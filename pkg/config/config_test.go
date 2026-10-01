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

func TestLoadServiceConfig_OmittedFields(t *testing.T) {
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

	// Omitted fields stay unset so reconciliation does not change live values.
	if cfg.LaunchType != "" || cfg.LaunchTypeConfigured {
		t.Errorf("omitted LaunchType: got %q (configured=%t), want unset", cfg.LaunchType, cfg.LaunchTypeConfigured)
	}
	if cfg.DesiredCount != 0 || cfg.DesiredCountConfigured {
		t.Errorf("omitted DesiredCount: got %d (configured=%t), want unset", cfg.DesiredCount, cfg.DesiredCountConfigured)
	}
	if cfg.SchedulingStrategy != "REPLICA" {
		t.Errorf("SchedulingStrategy default: got %q, want REPLICA", cfg.SchedulingStrategy)
	}
}

func TestLoadServiceConfig_DaemonScheduling(t *testing.T) {
	path := writeTemp(t, "name: agent\ncluster: prod\ntaskDefinition: agent:1\nschedulingStrategy: DAEMON\n")
	cfg, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SchedulingStrategy != "DAEMON" || cfg.DesiredCount != 0 {
		t.Fatalf("daemon service config = %#v", cfg)
	}
}

func TestLoadServiceConfig_RejectsDaemonDesiredCountAndScaling(t *testing.T) {
	for _, input := range []string{
		"name: agent\ncluster: prod\ntaskDefinition: agent:1\nschedulingStrategy: DAEMON\ndesiredCount: 1\n",
		"name: agent\ncluster: prod\ntaskDefinition: agent:1\nschedulingStrategy: DAEMON\nautoScaling:\n  minCapacity: 0\n  maxCapacity: 2\n",
	} {
		if _, err := LoadServiceConfig(writeTemp(t, input)); err == nil {
			t.Fatalf("expected invalid daemon config to fail: %s", input)
		}
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

func TestLoadServiceConfig_AutoScaling(t *testing.T) {
	path := writeTemp(t, "name: web\ncluster: prod\ntaskDefinition: web:1\nautoScaling:\n  minCapacity: 2\n  maxCapacity: 8\n  metric: Memory\n  targetValue: 70\n")
	cfg, err := LoadServiceConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AutoScaling == nil || cfg.AutoScaling.MinCapacity != 2 || cfg.AutoScaling.MaxCapacity != 8 || cfg.AutoScaling.Metric != "Memory" {
		t.Fatalf("autoScaling config not loaded: %#v", cfg.AutoScaling)
	}
}

func TestLoadServiceConfig_RejectsInvalidAutoScalingRange(t *testing.T) {
	path := writeTemp(t, "name: web\ncluster: prod\ntaskDefinition: web:1\nautoScaling:\n  minCapacity: 9\n  maxCapacity: 2\n")
	if _, err := LoadServiceConfig(path); err == nil {
		t.Fatal("expected invalid capacity range to fail")
	}
}

func TestLoadExpressServiceConfig_ImageService(t *testing.T) {
	path := writeTemp(t, "serviceName: api\ninfrastructureRoleArn: arn:aws:iam::123456789012:role/express\nimage: nginx:latest\ncontainerPort: 8080\nscalingMetric: AVERAGE_CPU\n")
	cfg, err := LoadExpressServiceConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServiceName != "api" || cfg.Image != "nginx:latest" || cfg.ContainerPort != 8080 {
		t.Fatalf("Express config not loaded: %#v", cfg)
	}
}

func TestLoadExpressServiceConfig_RequiresExactlyOneImageOrTaskDefinition(t *testing.T) {
	for _, content := range []string{
		"serviceName: api\ninfrastructureRoleArn: role\n",
		"serviceName: api\ninfrastructureRoleArn: role\nimage: nginx\ntaskDefinitionArn: arn:aws:ecs:task-definition/api:1\n",
	} {
		if _, err := LoadExpressServiceConfig(writeTemp(t, content)); err == nil {
			t.Fatalf("expected invalid Express config to fail: %s", content)
		}
	}
}
