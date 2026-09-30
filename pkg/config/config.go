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
	Name                            string                           `yaml:"name"`
	Region                          string                           `yaml:"region"`
	CapacityProviders               []string                         `yaml:"capacityProviders"`
	DefaultCapacityProviderStrategy []CapacityProviderStrategyConfig `yaml:"defaultCapacityProviderStrategy"`
	ServiceConnectDefaultsNamespace string                           `yaml:"serviceConnectDefaultsNamespace"`
	Tags                            map[string]string                `yaml:"tags"`
}

// NetworkConfig holds VPC networking settings for a service.
type NetworkConfig struct {
	Subnets        []string `yaml:"subnets"`
	SecurityGroups []string `yaml:"securityGroups"`
	AssignPublicIP string   `yaml:"assignPublicIp"` // ENABLED | DISABLED
}

type CapacityProviderStrategyConfig struct {
	CapacityProvider string `yaml:"capacityProvider"`
	Weight           int32  `yaml:"weight"`
	Base             int32  `yaml:"base"`
}

type DeploymentControllerConfig struct {
	Type string `yaml:"type"`
}
type DeploymentCircuitBreakerConfig struct {
	Enable   bool `yaml:"enable"`
	Rollback bool `yaml:"rollback"`
}
type DeploymentAlarmConfig struct {
	Name     string `yaml:"name"`
	Enable   bool   `yaml:"enable"`
	Rollback bool   `yaml:"rollback"`
}
type DeploymentConfigurationConfig struct {
	MaximumPercent           int32                           `yaml:"maximumPercent"`
	MinimumHealthyPercent    int32                           `yaml:"minimumHealthyPercent"`
	DeploymentCircuitBreaker *DeploymentCircuitBreakerConfig `yaml:"deploymentCircuitBreaker"`
	Alarms                   []DeploymentAlarmConfig         `yaml:"alarms"`
	Strategy                 string                          `yaml:"strategy"`
	BakeTimeInMinutes        int32                           `yaml:"bakeTimeInMinutes"`
}
type LoadBalancerConfig struct {
	TargetGroupARN string `yaml:"targetGroupArn"`
	ContainerName  string `yaml:"containerName"`
	ContainerPort  int32  `yaml:"containerPort"`
}
type ServiceRegistryConfig struct {
	RegistryARN   string `yaml:"registryArn"`
	Port          int32  `yaml:"port"`
	ContainerName string `yaml:"containerName"`
	ContainerPort int32  `yaml:"containerPort"`
}
type PlacementConstraintConfig struct {
	Type       string `yaml:"type"`
	Expression string `yaml:"expression"`
}
type PlacementStrategyConfig struct {
	Type  string `yaml:"type"`
	Field string `yaml:"field"`
}

type ServiceConnectClientAliasConfig struct {
	DNSName string `yaml:"dnsName"`
	Port    int32  `yaml:"port"`
}
type ServiceConnectServiceConfig struct {
	PortName            string                            `yaml:"portName"`
	DiscoveryName       string                            `yaml:"discoveryName"`
	IngressPortOverride int32                             `yaml:"ingressPortOverride"`
	ClientAliases       []ServiceConnectClientAliasConfig `yaml:"clientAliases"`
}
type ServiceConnectConfig struct {
	Enabled   bool                          `yaml:"enabled"`
	Namespace string                        `yaml:"namespace"`
	Services  []ServiceConnectServiceConfig `yaml:"services"`
}
type ServiceAutoScalingConfig struct {
	MinCapacity int32   `yaml:"minCapacity"`
	MaxCapacity int32   `yaml:"maxCapacity"`
	Metric      string  `yaml:"metric"`
	TargetValue float64 `yaml:"targetValue"`
}

// ExpressServiceConfig describes an ECS Express Mode web service. ECS creates
// and manages the supporting load balancer, URL, scaling, logs, and alarms.
type ExpressServiceConfig struct {
	ServiceName           string                              `yaml:"serviceName"`
	Cluster               string                              `yaml:"cluster"`
	InfrastructureRoleARN string                              `yaml:"infrastructureRoleArn"`
	ExecutionRoleARN      string                              `yaml:"executionRoleArn"`
	TaskRoleARN           string                              `yaml:"taskRoleArn"`
	TaskDefinitionARN     string                              `yaml:"taskDefinitionArn"`
	Image                 string                              `yaml:"image"`
	ContainerPort         int32                               `yaml:"containerPort"`
	Command               []string                            `yaml:"command"`
	Environment           map[string]string                   `yaml:"environment"`
	Secrets               map[string]string                   `yaml:"secrets"`
	AWSLogsConfiguration  *ExpressAWSLogsConfig               `yaml:"awsLogsConfiguration"`
	RepositoryCredentials *ExpressRepositoryCredentialsConfig `yaml:"repositoryCredentials"`
	CPU                   string                              `yaml:"cpu"`
	Memory                string                              `yaml:"memory"`
	CPUArchitecture       string                              `yaml:"cpuArchitecture"`
	HealthCheckPath       string                              `yaml:"healthCheckPath"`
	Subnets               []string                            `yaml:"subnets"`
	SecurityGroups        []string                            `yaml:"securityGroups"`
	MinTaskCount          int32                               `yaml:"minTaskCount"`
	MaxTaskCount          int32                               `yaml:"maxTaskCount"`
	ScalingMetric         string                              `yaml:"scalingMetric"`
	ScalingTargetValue    int32                               `yaml:"scalingTargetValue"`
	Tags                  map[string]string                   `yaml:"tags"`
}

type ExpressAWSLogsConfig struct {
	LogGroup        string `yaml:"logGroup"`
	LogStreamPrefix string `yaml:"logStreamPrefix"`
}

type ExpressRepositoryCredentialsConfig struct {
	CredentialsParameter string `yaml:"credentialsParameter"`
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
	Name                          string                           `yaml:"name"`
	Cluster                       string                           `yaml:"cluster"`
	TaskDefinition                string                           `yaml:"taskDefinition"`
	LaunchType                    string                           `yaml:"launchType"`
	SchedulingStrategy            string                           `yaml:"schedulingStrategy"`
	DesiredCount                  int32                            `yaml:"desiredCount"`
	NetworkConfig                 *NetworkConfig                   `yaml:"network"`
	Tags                          map[string]string                `yaml:"tags"`
	CapacityProviderStrategy      []CapacityProviderStrategyConfig `yaml:"capacityProviderStrategy"`
	DeploymentController          *DeploymentControllerConfig      `yaml:"deploymentController"`
	DeploymentConfiguration       *DeploymentConfigurationConfig   `yaml:"deploymentConfiguration"`
	LoadBalancers                 []LoadBalancerConfig             `yaml:"loadBalancers"`
	ServiceRegistries             []ServiceRegistryConfig          `yaml:"serviceRegistries"`
	HealthCheckGracePeriodSeconds int32                            `yaml:"healthCheckGracePeriodSeconds"`
	EnableExecuteCommand          *bool                            `yaml:"enableExecuteCommand"`
	EnableECSManagedTags          *bool                            `yaml:"enableECSManagedTags"`
	PropagateTags                 string                           `yaml:"propagateTags"`
	PlatformVersion               string                           `yaml:"platformVersion"`
	PlacementConstraints          []PlacementConstraintConfig      `yaml:"placementConstraints"`
	PlacementStrategy             []PlacementStrategyConfig        `yaml:"placementStrategy"`
	ServiceConnect                *ServiceConnectConfig            `yaml:"serviceConnect"`
	AutoScaling                   *ServiceAutoScalingConfig        `yaml:"autoScaling"`
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
	if _, ok := raw["serviceName"]; ok {
		return "express-service", nil
	}
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
	if cfg.SchedulingStrategy == "" {
		cfg.SchedulingStrategy = "REPLICA"
	}
	if cfg.SchedulingStrategy != "REPLICA" && cfg.SchedulingStrategy != "DAEMON" {
		return nil, fmt.Errorf("config file %q: schedulingStrategy must be REPLICA or DAEMON", path)
	}
	if cfg.SchedulingStrategy == "DAEMON" && cfg.DesiredCount > 0 {
		return nil, fmt.Errorf("config file %q: desiredCount cannot be set for DAEMON services", path)
	}
	if cfg.SchedulingStrategy == "DAEMON" && cfg.AutoScaling != nil {
		return nil, fmt.Errorf("config file %q: autoScaling cannot be configured for DAEMON services", path)
	}
	if cfg.SchedulingStrategy == "REPLICA" && cfg.DesiredCount == 0 {
		cfg.DesiredCount = 1
	}
	if cfg.AutoScaling != nil {
		if cfg.AutoScaling.MinCapacity < 0 || cfg.AutoScaling.MaxCapacity <= 0 || cfg.AutoScaling.MinCapacity > cfg.AutoScaling.MaxCapacity {
			return nil, fmt.Errorf("config file %q: autoScaling requires 0 <= minCapacity <= maxCapacity and maxCapacity > 0", path)
		}
		if cfg.AutoScaling.TargetValue < 0 {
			return nil, fmt.Errorf("config file %q: autoScaling.targetValue must be positive", path)
		}
		metric := cfg.AutoScaling.Metric
		if metric != "" && metric != "CPU" && metric != "Memory" && metric != "cpu" && metric != "memory" {
			return nil, fmt.Errorf("config file %q: autoScaling.metric must be CPU or Memory", path)
		}
	}

	return &cfg, nil
}

// LoadExpressServiceConfig reads the configuration for an ECS Express Mode service.
func LoadExpressServiceConfig(path string) (*ExpressServiceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}
	var cfg ExpressServiceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("config file %q: 'serviceName' is required", path)
	}
	if cfg.InfrastructureRoleARN == "" {
		return nil, fmt.Errorf("config file %q: 'infrastructureRoleArn' is required", path)
	}
	if cfg.TaskDefinitionARN == "" && cfg.Image == "" {
		return nil, fmt.Errorf("config file %q: either 'taskDefinitionArn' or 'image' is required", path)
	}
	if cfg.TaskDefinitionARN != "" && cfg.Image != "" {
		return nil, fmt.Errorf("config file %q: set either 'taskDefinitionArn' or 'image', not both", path)
	}
	if cfg.TaskDefinitionARN != "" && (cfg.ExecutionRoleARN != "" || cfg.TaskRoleARN != "" || cfg.CPU != "" || cfg.Memory != "" || cfg.CPUArchitecture != "" || cfg.Command != nil || cfg.Environment != nil || cfg.Secrets != nil || cfg.AWSLogsConfiguration != nil || cfg.RepositoryCredentials != nil || cfg.ContainerPort != 0) {
		return nil, fmt.Errorf("config file %q: taskDefinitionArn cannot be combined with task/container configuration fields", path)
	}
	if cfg.ContainerPort < 0 || cfg.ContainerPort > 65535 {
		return nil, fmt.Errorf("config file %q: containerPort must be between 1 and 65535", path)
	}
	if cfg.AWSLogsConfiguration != nil && (cfg.AWSLogsConfiguration.LogGroup == "" || cfg.AWSLogsConfiguration.LogStreamPrefix == "") {
		return nil, fmt.Errorf("config file %q: awsLogsConfiguration requires logGroup and logStreamPrefix", path)
	}
	if cfg.RepositoryCredentials != nil && cfg.RepositoryCredentials.CredentialsParameter == "" {
		return nil, fmt.Errorf("config file %q: repositoryCredentials.credentialsParameter is required", path)
	}
	if cfg.MinTaskCount < 0 || cfg.MaxTaskCount < 0 || (cfg.MaxTaskCount > 0 && cfg.MinTaskCount > cfg.MaxTaskCount) {
		return nil, fmt.Errorf("config file %q: scaling requires 0 <= minTaskCount <= maxTaskCount", path)
	}
	if cfg.ScalingTargetValue < 0 {
		return nil, fmt.Errorf("config file %q: scalingTargetValue cannot be negative", path)
	}
	if cfg.ScalingMetric != "" && cfg.ScalingMetric != "AVERAGE_CPU" && cfg.ScalingMetric != "AVERAGE_MEMORY" && cfg.ScalingMetric != "REQUEST_COUNT_PER_TARGET" {
		return nil, fmt.Errorf("config file %q: scalingMetric must be AVERAGE_CPU, AVERAGE_MEMORY, or REQUEST_COUNT_PER_TARGET", path)
	}
	return &cfg, nil
}
