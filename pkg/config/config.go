package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ClusterConfig represents the YAML config for creating an ECS cluster.
//
// Example cluster.yaml:
//
//	name: my-cluster
//	region: us-east-1
//	capacityProviders:
//	  - FARGATE
//	  - FARGATE_SPOT
//	tags:
//	  env: production
//	  team: platform
type ClusterConfig struct {
	Name              string            `yaml:"name"`
	Region            string            `yaml:"region"`
	CapacityProviders []string          `yaml:"capacityProviders"`
	Tags              map[string]string `yaml:"tags"`
}

// NetworkConfig holds VPC networking settings for a service.
type NetworkConfig struct {
	Subnets        []string `yaml:"subnets"`
	SecurityGroups []string `yaml:"securityGroups"`
	AssignPublicIP string   `yaml:"assignPublicIp"` // ENABLED | DISABLED
}

// ServiceConfig represents the YAML config for creating an ECS service.
//
// Example service.yaml:
//
//	name: my-service
//	cluster: my-cluster
//	taskDefinition: my-task:3
//	launchType: FARGATE        # FARGATE | EC2
//	desiredCount: 2
//	network:
//	  subnets:
//	    - subnet-abc123
//	  securityGroups:
//	    - sg-abc123
//	  assignPublicIp: ENABLED
//	tags:
//	  env: production
type ServiceConfig struct {
	Name           string            `yaml:"name"`
	Cluster        string            `yaml:"cluster"`
	TaskDefinition string            `yaml:"taskDefinition"`
	LaunchType     string            `yaml:"launchType"`
	DesiredCount   int32             `yaml:"desiredCount"`
	NetworkConfig  *NetworkConfig    `yaml:"network"`
	Tags           map[string]string `yaml:"tags"`
}

// DetectResourceType reads a YAML file and returns "cluster" or "service"
// based on which required fields are present.
func DetectResourceType(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file %q: %w", path, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parsing file %q: %w", path, err)
	}

	// A service config always has a "cluster" field; a cluster config does not.
	if _, ok := raw["cluster"]; ok {
		return "service", nil
	}
	if _, ok := raw["name"]; ok {
		return "cluster", nil
	}
	return "", fmt.Errorf("cannot detect resource type in %q — file must have 'name' (cluster) or 'cluster' (service) field", path)
}

// LoadClusterConfig reads and parses a cluster YAML config file.
func LoadClusterConfig(path string) (*ClusterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg ClusterConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("config file %q: 'name' is required", path)
	}

	return &cfg, nil
}

// LoadServiceConfig reads and parses a service YAML config file.
func LoadServiceConfig(path string) (*ServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	var cfg ServiceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("config file %q: 'name' is required", path)
	}
	if cfg.Cluster == "" {
		return nil, fmt.Errorf("config file %q: 'cluster' is required", path)
	}
	if cfg.TaskDefinition == "" {
		return nil, fmt.Errorf("config file %q: 'taskDefinition' is required", path)
	}
	if cfg.LaunchType == "" {
		cfg.LaunchType = "FARGATE"
	}
	if cfg.DesiredCount == 0 {
		cfg.DesiredCount = 1
	}

	return &cfg, nil
}
