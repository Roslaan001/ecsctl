package aws

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
	"github.com/roslaan001/ecsctl/pkg/state"
	"gopkg.in/yaml.v3"
)

// CreateExpressService provisions an ECS Express Mode service and its managed ingress,
// scaling, logging, and monitoring resources.
func (c *Client) CreateExpressService(ctx context.Context, cfg *ecscfg.ExpressServiceConfig) (string, error) {
	in := &ecs.CreateExpressGatewayServiceInput{
		ServiceName: aws.String(cfg.ServiceName), Cluster: optionalString(cfg.Cluster),
		InfrastructureRoleArn: aws.String(cfg.InfrastructureRoleARN), Tags: toECSTags(cfg.Tags),
	}
	if cfg.TaskDefinitionARN != "" {
		in.TaskDefinitionArn = aws.String(cfg.TaskDefinitionARN)
	} else {
		in.PrimaryContainer = expressContainer(cfg)
		in.ExecutionRoleArn = optionalString(cfg.ExecutionRoleARN)
		in.TaskRoleArn = optionalString(cfg.TaskRoleARN)
		in.Cpu = optionalString(cfg.CPU)
		in.Memory = optionalString(cfg.Memory)
		in.CpuArchitecture = types.ExpressCpuArchitecture(cfg.CPUArchitecture)
	}
	if cfg.HealthCheckPath != "" {
		in.HealthCheckPath = aws.String(cfg.HealthCheckPath)
	}
	if len(cfg.Subnets) > 0 || len(cfg.SecurityGroups) > 0 {
		in.NetworkConfiguration = &types.ExpressGatewayServiceNetworkConfiguration{Subnets: cfg.Subnets, SecurityGroups: cfg.SecurityGroups}
	}
	if cfg.MinTaskCount != 0 || cfg.MaxTaskCount != 0 || cfg.ScalingMetric != "" || cfg.ScalingTargetValue != 0 {
		in.ScalingTarget = expressScaling(cfg)
	}
	out, err := c.ecs.CreateExpressGatewayService(ctx, in)
	if err != nil {
		return "", err
	}
	if out.Service == nil {
		return "", fmt.Errorf("ECS returned no Express service")
	}
	return aws.ToString(out.Service.ServiceArn), nil
}

func (c *Client) UpdateExpressService(ctx context.Context, arn string, cfg *ecscfg.ExpressServiceConfig) error {
	in := &ecs.UpdateExpressGatewayServiceInput{ServiceArn: aws.String(arn)}
	if cfg.TaskDefinitionARN != "" {
		in.TaskDefinitionArn = aws.String(cfg.TaskDefinitionARN)
	} else {
		in.PrimaryContainer = expressContainer(cfg)
		if cfg.ExecutionRoleARN != "" {
			in.ExecutionRoleArn = aws.String(cfg.ExecutionRoleARN)
		}
		if cfg.TaskRoleARN != "" {
			in.TaskRoleArn = aws.String(cfg.TaskRoleARN)
		}
		if cfg.CPU != "" {
			in.Cpu = aws.String(cfg.CPU)
		}
		if cfg.Memory != "" {
			in.Memory = aws.String(cfg.Memory)
		}
		if cfg.CPUArchitecture != "" {
			in.CpuArchitecture = types.ExpressCpuArchitecture(cfg.CPUArchitecture)
		}
	}
	if cfg.HealthCheckPath != "" {
		in.HealthCheckPath = aws.String(cfg.HealthCheckPath)
	}
	if len(cfg.Subnets) > 0 || len(cfg.SecurityGroups) > 0 {
		in.NetworkConfiguration = &types.ExpressGatewayServiceNetworkConfiguration{Subnets: cfg.Subnets, SecurityGroups: cfg.SecurityGroups}
	}
	if cfg.MinTaskCount != 0 || cfg.MaxTaskCount != 0 || cfg.ScalingMetric != "" || cfg.ScalingTargetValue != 0 {
		in.ScalingTarget = expressScaling(cfg)
	}
	_, err := c.ecs.UpdateExpressGatewayService(ctx, in)
	return err
}

// ReconcileExpressService compares configured fields with the active revision
// and updates only when a difference is present. Unspecified fields are left alone.
func (c *Client) ReconcileExpressService(ctx context.Context, arn string, cfg *ecscfg.ExpressServiceConfig, dryRun bool) (bool, error) {
	service, err := c.DescribeExpressService(ctx, arn)
	if err != nil {
		return false, fmt.Errorf("describing Express service: %w", err)
	}
	if service == nil {
		return false, fmt.Errorf("express service %q not found", arn)
	}
	if cfg.ServiceName != "" && aws.ToString(service.ServiceName) != cfg.ServiceName {
		return false, fmt.Errorf("express service name %q cannot be changed to %q", aws.ToString(service.ServiceName), cfg.ServiceName)
	}
	if cfg.Cluster != "" && aws.ToString(service.Cluster) != cfg.Cluster {
		return false, fmt.Errorf("Express service cluster %q cannot be changed to %q", aws.ToString(service.Cluster), cfg.Cluster)
	}
	if cfg.InfrastructureRoleARN != "" && aws.ToString(service.InfrastructureRoleArn) != cfg.InfrastructureRoleARN {
		return false, fmt.Errorf("Express infrastructure role cannot be changed in place; replace the service")
	}
	revision := activeExpressConfiguration(service)
	if revision == nil {
		return false, fmt.Errorf("Express service %q has no active configuration to compare", arn)
	}
	configurationDrift := expressConfigurationDrift(revision, cfg)
	tagsDrift := cfg.Tags != nil && !tagsEqual(service.Tags, cfg.Tags)
	drift := configurationDrift || tagsDrift
	if !drift {
		return false, nil
	}
	if dryRun {
		fmt.Printf("[dry-run] Would reconcile configured fields for Express service %q.\n", cfg.ServiceName)
		return true, nil
	}
	if configurationDrift {
		if err := c.UpdateExpressService(ctx, arn, cfg); err != nil {
			return false, fmt.Errorf("updating Express service: %w", err)
		}
	}
	if tagsDrift {
		if err := c.reconcileTags(ctx, aws.ToString(service.ServiceArn), service.Tags, cfg.Tags); err != nil {
			return false, fmt.Errorf("reconciling Express service tags: %w", err)
		}
	}
	fmt.Printf("✓ Express service %q updated.\n", cfg.ServiceName)
	return true, nil
}

func activeExpressConfiguration(service *types.ECSExpressGatewayService) *types.ExpressGatewayServiceConfiguration {
	for i := range service.ActiveConfigurations {
		if aws.ToString(service.ActiveConfigurations[i].ServiceRevisionArn) == aws.ToString(service.CurrentDeployment) {
			return &service.ActiveConfigurations[i]
		}
	}
	if len(service.ActiveConfigurations) > 0 {
		return &service.ActiveConfigurations[0]
	}
	return nil
}

func expressConfigurationDrift(current *types.ExpressGatewayServiceConfiguration, cfg *ecscfg.ExpressServiceConfig) bool {
	if cfg.TaskDefinitionARN != "" {
		if taskDefinitionRef(aws.ToString(current.TaskDefinitionArn)) != taskDefinitionRef(cfg.TaskDefinitionARN) {
			return true
		}
	} else {
		container := current.PrimaryContainer
		if container == nil || aws.ToString(container.Image) != cfg.Image {
			return true
		}
		if cfg.ContainerPort != 0 && aws.ToInt32(container.ContainerPort) != cfg.ContainerPort {
			return true
		}
		if cfg.Command != nil && !reflect.DeepEqual(container.Command, cfg.Command) {
			return true
		}
		if cfg.Environment != nil && !reflect.DeepEqual(expressEnvironmentMap(container.Environment), cfg.Environment) {
			return true
		}
		if cfg.Secrets != nil && !reflect.DeepEqual(expressSecretsMap(container.Secrets), cfg.Secrets) {
			return true
		}
		if cfg.AWSLogsConfiguration != nil && (container.AwsLogsConfiguration == nil ||
			aws.ToString(container.AwsLogsConfiguration.LogGroup) != cfg.AWSLogsConfiguration.LogGroup ||
			aws.ToString(container.AwsLogsConfiguration.LogStreamPrefix) != cfg.AWSLogsConfiguration.LogStreamPrefix) {
			return true
		}
		if cfg.RepositoryCredentials != nil && (container.RepositoryCredentials == nil ||
			aws.ToString(container.RepositoryCredentials.CredentialsParameter) != cfg.RepositoryCredentials.CredentialsParameter) {
			return true
		}
	}
	if cfg.ExecutionRoleARN != "" && aws.ToString(current.ExecutionRoleArn) != cfg.ExecutionRoleARN {
		return true
	}
	if cfg.TaskRoleARN != "" && aws.ToString(current.TaskRoleArn) != cfg.TaskRoleARN {
		return true
	}
	if cfg.CPU != "" && aws.ToString(current.Cpu) != cfg.CPU {
		return true
	}
	if cfg.Memory != "" && aws.ToString(current.Memory) != cfg.Memory {
		return true
	}
	if cfg.CPUArchitecture != "" && string(current.CpuArchitecture) != cfg.CPUArchitecture {
		return true
	}
	if cfg.HealthCheckPath != "" && aws.ToString(current.HealthCheckPath) != cfg.HealthCheckPath {
		return true
	}
	if len(cfg.Subnets) > 0 && (current.NetworkConfiguration == nil || !stringSlicesEqual(current.NetworkConfiguration.Subnets, cfg.Subnets)) {
		return true
	}
	if len(cfg.SecurityGroups) > 0 && (current.NetworkConfiguration == nil || !stringSlicesEqual(current.NetworkConfiguration.SecurityGroups, cfg.SecurityGroups)) {
		return true
	}
	if cfg.MinTaskCount != 0 || cfg.MaxTaskCount != 0 || cfg.ScalingMetric != "" || cfg.ScalingTargetValue != 0 {
		if current.ScalingTarget == nil {
			return true
		}
		if cfg.MinTaskCount != 0 && aws.ToInt32(current.ScalingTarget.MinTaskCount) != cfg.MinTaskCount {
			return true
		}
		if cfg.MaxTaskCount != 0 && aws.ToInt32(current.ScalingTarget.MaxTaskCount) != cfg.MaxTaskCount {
			return true
		}
		metric := cfg.ScalingMetric
		if metric == "" {
			metric = string(types.ExpressGatewayServiceScalingMetricAverageCPUUtilization)
		}
		currentMetric := string(current.ScalingTarget.AutoScalingMetric)
		if currentMetric == "" {
			currentMetric = string(types.ExpressGatewayServiceScalingMetricAverageCPUUtilization)
		}
		if currentMetric != metric {
			return true
		}
		target := cfg.ScalingTargetValue
		if target == 0 {
			target = 60
		}
		currentTarget := aws.ToInt32(current.ScalingTarget.AutoScalingTargetValue)
		if currentTarget == 0 {
			currentTarget = 60
		}
		if currentTarget != target {
			return true
		}
	}
	return false
}

func expressEnvironmentMap(values []types.KeyValuePair) map[string]string {
	result := make(map[string]string, len(values))
	for _, item := range values {
		result[aws.ToString(item.Name)] = aws.ToString(item.Value)
	}
	return result
}

func expressSecretsMap(values []types.Secret) map[string]string {
	result := make(map[string]string, len(values))
	for _, item := range values {
		result[aws.ToString(item.Name)] = aws.ToString(item.ValueFrom)
	}
	return result
}

func (c *Client) DescribeExpressService(ctx context.Context, arn string) (*types.ECSExpressGatewayService, error) {
	out, err := c.ecs.DescribeExpressGatewayService(ctx, &ecs.DescribeExpressGatewayServiceInput{ServiceArn: aws.String(arn), Include: []types.ExpressGatewayServiceInclude{types.ExpressGatewayServiceIncludeTags}})
	if err != nil {
		return nil, err
	}
	return out.Service, nil
}

// ListExpressServices returns ECS-managed Express services in the requested cluster.
// When cluster is empty, ECS lists services in the default cluster.
func (c *Client) ListExpressServices(ctx context.Context, cluster string) ([]types.ECSExpressGatewayService, error) {
	var token *string
	var arns []string
	for {
		page, err := c.ecs.ListServices(ctx, &ecs.ListServicesInput{
			Cluster: optionalString(cluster), NextToken: token,
			ResourceManagementType: types.ResourceManagementTypeEcs,
		})
		if err != nil {
			return nil, fmt.Errorf("listing Express services: %w", err)
		}
		arns = append(arns, page.ServiceArns...)
		if page.NextToken == nil {
			break
		}
		token = page.NextToken
	}

	services := make([]types.ECSExpressGatewayService, 0, len(arns))
	for _, arn := range arns {
		service, err := c.DescribeExpressService(ctx, arn)
		if err != nil {
			return nil, fmt.Errorf("describing Express service %q: %w", arn, err)
		}
		if service != nil {
			services = append(services, *service)
		}
	}
	return services, nil
}

// DescribeExpressServiceResource imports an Express service with its active
// configuration snapshot so state import can capture a reusable YAML baseline.
func (c *Client) DescribeExpressServiceResource(ctx context.Context, arn string) (*state.Resource, error) {
	service, err := c.DescribeExpressService(ctx, arn)
	if err != nil {
		return nil, fmt.Errorf("describing Express service: %w", err)
	}
	if service == nil {
		return nil, fmt.Errorf("Express service %q not found", arn)
	}
	cfg := expressConfigFromService(service)
	serialized, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("serializing Express service configuration: %w", err)
	}
	return &state.Resource{
		Type: state.ResourceTypeExpressService, Name: aws.ToString(service.ServiceName), ARN: aws.ToString(service.ServiceArn),
		Cluster: aws.ToString(service.Cluster), CreatedBy: "imported", Configuration: string(serialized),
	}, nil
}

func expressConfigFromService(service *types.ECSExpressGatewayService) *ecscfg.ExpressServiceConfig {
	cfg := &ecscfg.ExpressServiceConfig{
		ServiceName: aws.ToString(service.ServiceName), Cluster: aws.ToString(service.Cluster),
		InfrastructureRoleARN: aws.ToString(service.InfrastructureRoleArn),
	}
	if len(service.ActiveConfigurations) == 0 {
		return cfg
	}
	revision := service.ActiveConfigurations[0]
	if revision.TaskDefinitionArn != nil {
		cfg.TaskDefinitionARN = aws.ToString(revision.TaskDefinitionArn)
	} else {
		cfg.ExecutionRoleARN = aws.ToString(revision.ExecutionRoleArn)
		cfg.TaskRoleARN = aws.ToString(revision.TaskRoleArn)
		cfg.CPU = aws.ToString(revision.Cpu)
		cfg.Memory = aws.ToString(revision.Memory)
		cfg.CPUArchitecture = string(revision.CpuArchitecture)
		if container := revision.PrimaryContainer; container != nil {
			cfg.Image = aws.ToString(container.Image)
			cfg.ContainerPort = aws.ToInt32(container.ContainerPort)
			cfg.Command = container.Command
			cfg.Environment = make(map[string]string, len(container.Environment))
			for _, value := range container.Environment {
				cfg.Environment[aws.ToString(value.Name)] = aws.ToString(value.Value)
			}
			cfg.Secrets = make(map[string]string, len(container.Secrets))
			for _, value := range container.Secrets {
				cfg.Secrets[aws.ToString(value.Name)] = aws.ToString(value.ValueFrom)
			}
			if logs := container.AwsLogsConfiguration; logs != nil {
				cfg.AWSLogsConfiguration = &ecscfg.ExpressAWSLogsConfig{LogGroup: aws.ToString(logs.LogGroup), LogStreamPrefix: aws.ToString(logs.LogStreamPrefix)}
			}
			if credentials := container.RepositoryCredentials; credentials != nil {
				cfg.RepositoryCredentials = &ecscfg.ExpressRepositoryCredentialsConfig{CredentialsParameter: aws.ToString(credentials.CredentialsParameter)}
			}
		}
	}
	cfg.HealthCheckPath = aws.ToString(revision.HealthCheckPath)
	if network := revision.NetworkConfiguration; network != nil {
		cfg.Subnets = network.Subnets
		cfg.SecurityGroups = network.SecurityGroups
	}
	if scaling := revision.ScalingTarget; scaling != nil {
		cfg.MinTaskCount = aws.ToInt32(scaling.MinTaskCount)
		cfg.MaxTaskCount = aws.ToInt32(scaling.MaxTaskCount)
		cfg.ScalingMetric = string(scaling.AutoScalingMetric)
		cfg.ScalingTargetValue = aws.ToInt32(scaling.AutoScalingTargetValue)
	}
	cfg.Tags = make(map[string]string, len(service.Tags))
	for _, tag := range service.Tags {
		cfg.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return cfg
}

func (c *Client) DeleteExpressService(ctx context.Context, arn string) error {
	_, err := c.ecs.DeleteExpressGatewayService(ctx, &ecs.DeleteExpressGatewayServiceInput{ServiceArn: aws.String(arn)})
	return err
}

func (c *Client) WaitForExpressService(ctx context.Context, arn string) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		service, err := c.DescribeExpressService(ctx, arn)
		if err != nil {
			return err
		}
		if service != nil && service.Status != nil {
			fmt.Printf("  Express service status: %s\n", service.Status.StatusCode)
			if service.Status.StatusCode == types.ExpressGatewayServiceStatusCodeActive {
				return nil
			}
			if service.Status.StatusCode == types.ExpressGatewayServiceStatusCodeInactive {
				return fmt.Errorf("express service became inactive: %s", aws.ToString(service.Status.StatusReason))
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for Express service to become active")
		case <-ticker.C:
		}
	}
}

func expressContainer(cfg *ecscfg.ExpressServiceConfig) *types.ExpressGatewayContainer {
	container := &types.ExpressGatewayContainer{Image: aws.String(cfg.Image), Command: cfg.Command}
	if cfg.ContainerPort != 0 {
		container.ContainerPort = aws.Int32(cfg.ContainerPort)
	}
	for key, value := range cfg.Environment {
		container.Environment = append(container.Environment, types.KeyValuePair{Name: aws.String(key), Value: aws.String(value)})
	}
	for key, value := range cfg.Secrets {
		container.Secrets = append(container.Secrets, types.Secret{Name: aws.String(key), ValueFrom: aws.String(value)})
	}
	if cfg.AWSLogsConfiguration != nil {
		container.AwsLogsConfiguration = &types.ExpressGatewayServiceAwsLogsConfiguration{
			LogGroup:        aws.String(cfg.AWSLogsConfiguration.LogGroup),
			LogStreamPrefix: aws.String(cfg.AWSLogsConfiguration.LogStreamPrefix),
		}
	}
	if cfg.RepositoryCredentials != nil {
		container.RepositoryCredentials = &types.ExpressGatewayRepositoryCredentials{CredentialsParameter: aws.String(cfg.RepositoryCredentials.CredentialsParameter)}
	}
	return container
}

func expressScaling(cfg *ecscfg.ExpressServiceConfig) *types.ExpressGatewayScalingTarget {
	metric := cfg.ScalingMetric
	if metric == "" {
		metric = string(types.ExpressGatewayServiceScalingMetricAverageCPUUtilization)
	}
	value := cfg.ScalingTargetValue
	if value == 0 {
		value = 60
	}
	return &types.ExpressGatewayScalingTarget{MinTaskCount: optionalInt32(cfg.MinTaskCount), MaxTaskCount: optionalInt32(cfg.MaxTaskCount), AutoScalingMetric: types.ExpressGatewayServiceScalingMetric(metric), AutoScalingTargetValue: aws.Int32(value)}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}
func optionalInt32(value int32) *int32 {
	if value == 0 {
		return nil
	}
	return aws.Int32(value)
}
