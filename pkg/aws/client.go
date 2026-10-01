package aws

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	applicationautoscalingTypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
	"github.com/roslaan001/ecsctl/pkg/state"
	"gopkg.in/yaml.v3"
)

// ecsIface is the subset of the ECS SDK client used by Client.
// Defining it as an interface allows tests to inject a mock.
type ecsIface interface {
	CreateCluster(ctx context.Context, params *ecs.CreateClusterInput, optFns ...func(*ecs.Options)) (*ecs.CreateClusterOutput, error)
	UpdateCluster(ctx context.Context, params *ecs.UpdateClusterInput, optFns ...func(*ecs.Options)) (*ecs.UpdateClusterOutput, error)
	PutClusterCapacityProviders(ctx context.Context, params *ecs.PutClusterCapacityProvidersInput, optFns ...func(*ecs.Options)) (*ecs.PutClusterCapacityProvidersOutput, error)
	DeleteCluster(ctx context.Context, params *ecs.DeleteClusterInput, optFns ...func(*ecs.Options)) (*ecs.DeleteClusterOutput, error)
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	CreateService(ctx context.Context, params *ecs.CreateServiceInput, optFns ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error)
	CreateExpressGatewayService(ctx context.Context, params *ecs.CreateExpressGatewayServiceInput, optFns ...func(*ecs.Options)) (*ecs.CreateExpressGatewayServiceOutput, error)
	UpdateExpressGatewayService(ctx context.Context, params *ecs.UpdateExpressGatewayServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateExpressGatewayServiceOutput, error)
	DescribeExpressGatewayService(ctx context.Context, params *ecs.DescribeExpressGatewayServiceInput, optFns ...func(*ecs.Options)) (*ecs.DescribeExpressGatewayServiceOutput, error)
	DeleteExpressGatewayService(ctx context.Context, params *ecs.DeleteExpressGatewayServiceInput, optFns ...func(*ecs.Options)) (*ecs.DeleteExpressGatewayServiceOutput, error)
	DeleteService(ctx context.Context, params *ecs.DeleteServiceInput, optFns ...func(*ecs.Options)) (*ecs.DeleteServiceOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	RegisterTaskDefinition(ctx context.Context, params *ecs.RegisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error)
	RunTask(ctx context.Context, params *ecs.RunTaskInput, optFns ...func(*ecs.Options)) (*ecs.RunTaskOutput, error)
	StopTask(ctx context.Context, params *ecs.StopTaskInput, optFns ...func(*ecs.Options)) (*ecs.StopTaskOutput, error)
	TagResource(ctx context.Context, params *ecs.TagResourceInput, optFns ...func(*ecs.Options)) (*ecs.TagResourceOutput, error)
	UntagResource(ctx context.Context, params *ecs.UntagResourceInput, optFns ...func(*ecs.Options)) (*ecs.UntagResourceOutput, error)
}

type applicationAutoScalingIface interface {
	RegisterScalableTarget(ctx context.Context, params *applicationautoscaling.RegisterScalableTargetInput, optFns ...func(*applicationautoscaling.Options)) (*applicationautoscaling.RegisterScalableTargetOutput, error)
	PutScalingPolicy(ctx context.Context, params *applicationautoscaling.PutScalingPolicyInput, optFns ...func(*applicationautoscaling.Options)) (*applicationautoscaling.PutScalingPolicyOutput, error)
	DescribeScalableTargets(ctx context.Context, params *applicationautoscaling.DescribeScalableTargetsInput, optFns ...func(*applicationautoscaling.Options)) (*applicationautoscaling.DescribeScalableTargetsOutput, error)
	DescribeScalingPolicies(ctx context.Context, params *applicationautoscaling.DescribeScalingPoliciesInput, optFns ...func(*applicationautoscaling.Options)) (*applicationautoscaling.DescribeScalingPoliciesOutput, error)
}

// Client wraps the AWS ECS and CloudWatch Logs SDK clients.
type Client struct {
	ecs         ecsIface
	logs        *cloudwatchlogs.Client
	autoscaling applicationAutoScalingIface
	region      string
}

// LogsOptions holds options for fetching logs.
type LogsOptions struct {
	Cluster       string
	Service       string
	ContainerName string
	Tail          bool
	TailLines     bool
}

// ExecOptions holds options for exec into a task.
type ExecOptions struct {
	Cluster   string
	Service   string
	TaskID    string
	Container string
	Command   string
}

type RunTaskOptions struct {
	Cluster        string
	TaskDefinition string
	Count          int32
	Subnets        []string
	SecurityGroups []string
	AssignPublicIP string
}

func (c *Client) RegisterTaskDefinition(ctx context.Context, input *ecs.RegisterTaskDefinitionInput) (string, error) {
	out, err := c.ecs.RegisterTaskDefinition(ctx, input)
	if err != nil {
		return "", err
	}
	return aws.ToString(out.TaskDefinition.TaskDefinitionArn), nil
}

func (c *Client) RunTask(ctx context.Context, opts RunTaskOptions) ([]string, error) {
	input := &ecs.RunTaskInput{Cluster: aws.String(opts.Cluster), TaskDefinition: aws.String(opts.TaskDefinition), Count: aws.Int32(opts.Count), LaunchType: types.LaunchTypeFargate}
	if len(opts.Subnets) > 0 || len(opts.SecurityGroups) > 0 {
		input.NetworkConfiguration = &types.NetworkConfiguration{AwsvpcConfiguration: &types.AwsVpcConfiguration{Subnets: opts.Subnets, SecurityGroups: opts.SecurityGroups, AssignPublicIp: types.AssignPublicIp(opts.AssignPublicIP)}}
	}
	out, err := c.ecs.RunTask(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(out.Failures) > 0 {
		return nil, fmt.Errorf("running task failed: %s: %s", aws.ToString(out.Failures[0].Arn), aws.ToString(out.Failures[0].Reason))
	}
	arns := make([]string, 0, len(out.Tasks))
	for _, task := range out.Tasks {
		arns = append(arns, aws.ToString(task.TaskArn))
	}
	return arns, nil
}

func (c *Client) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	_, err := c.ecs.StopTask(ctx, &ecs.StopTaskInput{Cluster: aws.String(cluster), Task: aws.String(taskARN), Reason: aws.String(reason)})
	return err
}

// NewECSClient creates a new AWS client using the given region and profile.
// If region or profile are empty, the SDK falls back to env vars / shared config.
func NewECSClient(ctx context.Context, region, profile string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &Client{
		ecs:         ecs.NewFromConfig(cfg),
		logs:        cloudwatchlogs.NewFromConfig(cfg),
		autoscaling: applicationautoscaling.NewFromConfig(cfg),
		region:      cfg.Region,
	}, nil
}

// Region returns the resolved AWS region used by this client.
func (c *Client) Region() string { return c.region }

// ClusterARN returns the ARN for a named ECS cluster, regardless of status.
func (c *Client) ClusterARN(ctx context.Context, clusterName string) (string, error) {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: []string{clusterName}})
	if err != nil {
		return "", fmt.Errorf("describing cluster identity: %w", err)
	}
	if len(out.Clusters) == 0 || aws.ToString(out.Clusters[0].ClusterArn) == "" {
		return "", fmt.Errorf("cluster %q not found", clusterName)
	}
	return aws.ToString(out.Clusters[0].ClusterArn), nil
}

// ServiceARN returns the ARN for a named ECS service without querying optional
// side resources such as Application Auto Scaling.
func (c *Client) ServiceARN(ctx context.Context, clusterName, serviceName string) (string, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil {
		return "", fmt.Errorf("describing service identity: %w", err)
	}
	if len(out.Services) == 0 || aws.ToString(out.Services[0].ServiceArn) == "" {
		return "", fmt.Errorf("service %q not found in cluster %q", serviceName, clusterName)
	}
	return aws.ToString(out.Services[0].ServiceArn), nil
}

// CreateCluster creates an ECS cluster from config.
// Returns an error if a cluster with the same name already exists and is ACTIVE.
func (c *Client) CreateCluster(ctx context.Context, cfg *ecscfg.ClusterConfig) error {
	// Existence check — use AWS as source of truth
	existing, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{cfg.Name},
	})
	if err != nil {
		return fmt.Errorf("checking for existing cluster: %w", err)
	}
	for _, cl := range existing.Clusters {
		if aws.ToString(cl.ClusterName) == cfg.Name && cl.Status != nil && *cl.Status == "ACTIVE" {
			return fmt.Errorf("cluster %q already exists (status: ACTIVE) — use 'ecsctl delete cluster %s' to remove it first", cfg.Name, cfg.Name)
		}
	}

	input := &ecs.CreateClusterInput{
		ClusterName: aws.String(cfg.Name),
		Tags:        toECSTags(cfg.Tags),
	}

	// Only set capacity providers when explicitly specified — omitting them avoids
	// the service-linked role assumption that can fail on fresh accounts.
	if len(cfg.CapacityProviders) > 0 {
		input.CapacityProviders = cfg.CapacityProviders
	}
	input.DefaultCapacityProviderStrategy = toCapacityProviderStrategy(cfg.DefaultCapacityProviderStrategy)
	if cfg.ServiceConnectDefaultsNamespace != "" {
		input.ServiceConnectDefaults = &types.ClusterServiceConnectDefaultsRequest{Namespace: aws.String(cfg.ServiceConnectDefaultsNamespace)}
	}

	_, err = c.ecs.CreateCluster(ctx, input)
	return err
}

// ReconcileCluster applies only cluster fields explicitly configured in YAML.
func (c *Client) ReconcileCluster(ctx context.Context, cfg *ecscfg.ClusterConfig, dryRun bool) (bool, error) {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{cfg.Name},
		Include:  []types.ClusterField{types.ClusterFieldSettings, types.ClusterFieldTags},
	})
	if err != nil {
		return false, fmt.Errorf("describing cluster: %w", err)
	}
	var current *types.Cluster
	for i := range out.Clusters {
		if aws.ToString(out.Clusters[i].ClusterName) == cfg.Name && aws.ToString(out.Clusters[i].Status) == "ACTIVE" {
			current = &out.Clusters[i]
			break
		}
	}
	if current == nil {
		return false, fmt.Errorf("cluster %q not found or not ACTIVE", cfg.Name)
	}

	capacityProvidersConfigured := cfg.CapacityProviders != nil
	strategyConfigured := cfg.DefaultCapacityProviderStrategy != nil
	wantProviders := current.CapacityProviders
	if capacityProvidersConfigured {
		wantProviders = cfg.CapacityProviders
	}
	wantStrategy := current.DefaultCapacityProviderStrategy
	if strategyConfigured {
		wantStrategy = toCapacityProviderStrategy(cfg.DefaultCapacityProviderStrategy)
	}
	if strategyConfigured && !capacityProvidersConfigured {
		wantProviders = mergeCapacityProviders(wantProviders, cfg.DefaultCapacityProviderStrategy)
	}
	for _, item := range wantStrategy {
		provider := aws.ToString(item.CapacityProvider)
		if !containsString(wantProviders, provider) {
			return false, fmt.Errorf("default capacity provider %q is not included in desired capacityProviders", provider)
		}
	}
	capacityProviderDrift := (capacityProvidersConfigured || strategyConfigured) &&
		(!stringSlicesEqual(current.CapacityProviders, wantProviders) || !capacityProviderStrategiesEqual(current.DefaultCapacityProviderStrategy, wantStrategy))
	serviceConnectDrift := cfg.ServiceConnectDefaultsNamespace != "" &&
		(current.ServiceConnectDefaults == nil || aws.ToString(current.ServiceConnectDefaults.Namespace) != cfg.ServiceConnectDefaultsNamespace)
	tagsDrift := cfg.Tags != nil && !tagsEqual(current.Tags, cfg.Tags)
	changed := capacityProviderDrift || serviceConnectDrift || tagsDrift
	if !changed || dryRun {
		if dryRun && changed {
			fields := make([]string, 0, 3)
			if capacityProviderDrift {
				fields = append(fields, "capacityProviders/defaultCapacityProviderStrategy")
			}
			if serviceConnectDrift {
				fields = append(fields, "serviceConnectDefaultsNamespace")
			}
			if tagsDrift {
				fields = append(fields, "tags")
			}
			fmt.Printf("[dry-run] Cluster %q would update: %s.\n", cfg.Name, strings.Join(fields, ", "))
		}
		return changed, nil
	}

	if capacityProviderDrift {
		_, err := c.ecs.PutClusterCapacityProviders(ctx, &ecs.PutClusterCapacityProvidersInput{
			Cluster: aws.String(cfg.Name), CapacityProviders: wantProviders, DefaultCapacityProviderStrategy: wantStrategy,
		})
		if err != nil {
			return false, fmt.Errorf("updating cluster capacity providers: %w", err)
		}
	}
	if serviceConnectDrift {
		_, err := c.ecs.UpdateCluster(ctx, &ecs.UpdateClusterInput{
			Cluster:                aws.String(cfg.Name),
			ServiceConnectDefaults: &types.ClusterServiceConnectDefaultsRequest{Namespace: aws.String(cfg.ServiceConnectDefaultsNamespace)},
		})
		if err != nil {
			return false, fmt.Errorf("updating cluster Service Connect defaults: %w", err)
		}
	}
	if tagsDrift {
		if err := c.reconcileTags(ctx, aws.ToString(current.ClusterArn), current.Tags, cfg.Tags); err != nil {
			return false, fmt.Errorf("reconciling cluster tags: %w", err)
		}
	}
	fmt.Printf("✓ Cluster %q updated.\n", cfg.Name)
	return true, nil
}

func capacityProvidersFromStrategy(strategy []ecscfg.CapacityProviderStrategyConfig) []string {
	providers := make([]string, 0, len(strategy))
	seen := make(map[string]struct{}, len(strategy))
	for _, item := range strategy {
		if _, ok := seen[item.CapacityProvider]; ok || item.CapacityProvider == "" {
			continue
		}
		seen[item.CapacityProvider] = struct{}{}
		providers = append(providers, item.CapacityProvider)
	}
	return providers
}

func mergeCapacityProviders(existing []string, strategy []ecscfg.CapacityProviderStrategyConfig) []string {
	providers := append([]string(nil), existing...)
	for _, provider := range capacityProvidersFromStrategy(strategy) {
		if !containsString(providers, provider) {
			providers = append(providers, provider)
		}
	}
	return providers
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return reflect.DeepEqual(leftCopy, rightCopy)
}

func capacityProviderStrategiesEqual(left, right []types.CapacityProviderStrategyItem) bool {
	if len(left) != len(right) {
		return false
	}
	key := func(item types.CapacityProviderStrategyItem) string {
		return fmt.Sprintf("%s:%d:%d", aws.ToString(item.CapacityProvider), item.Weight, item.Base)
	}
	leftKeys, rightKeys := make([]string, 0, len(left)), make([]string, 0, len(right))
	for _, item := range left {
		leftKeys = append(leftKeys, key(item))
	}
	for _, item := range right {
		rightKeys = append(rightKeys, key(item))
	}
	sort.Strings(leftKeys)
	sort.Strings(rightKeys)
	return reflect.DeepEqual(leftKeys, rightKeys)
}

// DeleteCluster deletes an ECS cluster by name.
// Returns an error if the cluster has active services unless force is true,
// in which case all services are drained and deleted first.
func (c *Client) DeleteCluster(ctx context.Context, clusterName string, force bool) error {
	// Check for active services
	listOut, err := c.ecs.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("listing services in cluster: %w", err)
	}

	if len(listOut.ServiceArns) > 0 {
		if !force {
			// Describe services to show names
			descOut, _ := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: listOut.ServiceArns,
			})
			fmt.Printf("✗ Cluster %q has %d active service(s):\n", clusterName, len(listOut.ServiceArns))
			for _, svc := range descOut.Services {
				fmt.Printf("  - %s (%d running tasks)\n", aws.ToString(svc.ServiceName), svc.RunningCount)
			}
			return fmt.Errorf("refusing to delete cluster with active services — delete them first or use --force to delete all services automatically")
		}

		// --force: drain and delete all services first
		fmt.Printf("Draining and deleting %d service(s) in cluster %q...\n", len(listOut.ServiceArns), clusterName)
		descOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: listOut.ServiceArns,
		})
		if err != nil {
			return fmt.Errorf("describing services: %w", err)
		}
		for _, svc := range descOut.Services {
			svcName := aws.ToString(svc.ServiceName)
			fmt.Printf("  Deleting service %q...\n", svcName)
			if err := c.DeleteService(ctx, clusterName, svcName); err != nil {
				return fmt.Errorf("deleting service %q: %w", svcName, err)
			}
		}
	}

	_, err = c.ecs.DeleteCluster(ctx, &ecs.DeleteClusterInput{
		Cluster: aws.String(clusterName),
	})
	return err
}

// CreateService creates an ECS service from config.
// Returns an error if a service with the same name already exists in the cluster.
func (c *Client) CreateService(ctx context.Context, cfg *ecscfg.ServiceConfig) error {
	// Existence check
	existing, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Cluster),
		Services: []string{cfg.Name},
	})
	if err != nil {
		return fmt.Errorf("checking for existing service: %w", err)
	}
	for _, svc := range existing.Services {
		if aws.ToString(svc.ServiceName) == cfg.Name && aws.ToString(svc.Status) == "ACTIVE" {
			return fmt.Errorf("service %q already exists in cluster %q — use 'ecsctl delete service %s --cluster %s' to remove it first", cfg.Name, cfg.Cluster, cfg.Name, cfg.Cluster)
		}
	}

	schedulingStrategy := cfg.SchedulingStrategy
	if schedulingStrategy == "" {
		schedulingStrategy = "REPLICA"
	}
	launchType := cfg.LaunchType
	if launchType == "" && len(cfg.CapacityProviderStrategy) == 0 {
		launchType = "FARGATE"
	}
	input := &ecs.CreateServiceInput{
		Cluster:                 aws.String(cfg.Cluster),
		ServiceName:             aws.String(cfg.Name),
		TaskDefinition:          aws.String(cfg.TaskDefinition),
		LaunchType:              types.LaunchType(launchType),
		SchedulingStrategy:      types.SchedulingStrategy(schedulingStrategy),
		DeploymentController:    toDeploymentController(cfg.DeploymentController),
		DeploymentConfiguration: toDeploymentConfiguration(cfg.DeploymentConfiguration),
		LoadBalancers:           toLoadBalancers(cfg.LoadBalancers),
		ServiceRegistries:       toServiceRegistries(cfg.ServiceRegistries),
		PlacementConstraints:    toPlacementConstraints(cfg.PlacementConstraints),
		PlacementStrategy:       toPlacementStrategies(cfg.PlacementStrategy),
		Tags:                    toECSTags(cfg.Tags),
		PropagateTags:           types.PropagateTags(cfg.PropagateTags),
	}
	if cfg.DesiredCountConfigured || cfg.DesiredCount > 0 {
		input.DesiredCount = aws.Int32(cfg.DesiredCount)
	} else if schedulingStrategy == "REPLICA" {
		input.DesiredCount = aws.Int32(1)
	}
	if cfg.EnableECSManagedTags != nil {
		input.EnableECSManagedTags = *cfg.EnableECSManagedTags
	}
	if len(cfg.CapacityProviderStrategy) > 0 {
		input.CapacityProviderStrategy = toCapacityProviderStrategy(cfg.CapacityProviderStrategy)
		input.LaunchType = ""
	}
	if cfg.EnableExecuteCommand != nil {
		input.EnableExecuteCommand = *cfg.EnableExecuteCommand
	}
	if cfg.HealthCheckGracePeriodSeconds > 0 {
		input.HealthCheckGracePeriodSeconds = aws.Int32(cfg.HealthCheckGracePeriodSeconds)
	}
	if cfg.PlatformVersion != "" {
		input.PlatformVersion = aws.String(cfg.PlatformVersion)
	}
	if cfg.ServiceConnect != nil {
		input.ServiceConnectConfiguration = toServiceConnect(cfg.ServiceConnect)
	}

	if cfg.NetworkConfig != nil {
		input.NetworkConfiguration = &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        cfg.NetworkConfig.Subnets,
				SecurityGroups: cfg.NetworkConfig.SecurityGroups,
				AssignPublicIp: types.AssignPublicIp(cfg.NetworkConfig.AssignPublicIP),
			},
		}
	}

	_, err = c.ecs.CreateService(ctx, input)
	if err != nil {
		return err
	}
	if cfg.AutoScaling != nil {
		if err := c.configureServiceAutoScaling(ctx, cfg); err != nil {
			return fmt.Errorf("service created but configuring auto scaling failed: %w", err)
		}
	}
	return nil
}

func toCapacityProviderStrategy(items []ecscfg.CapacityProviderStrategyConfig) []types.CapacityProviderStrategyItem {
	out := make([]types.CapacityProviderStrategyItem, 0, len(items))
	for _, item := range items {
		v := types.CapacityProviderStrategyItem{CapacityProvider: aws.String(item.CapacityProvider)}
		if item.Weight > 0 {
			v.Weight = item.Weight
		}
		if item.Base > 0 {
			v.Base = item.Base
		}
		out = append(out, v)
	}
	return out
}

func toDeploymentController(cfg *ecscfg.DeploymentControllerConfig) *types.DeploymentController {
	if cfg == nil {
		return nil
	}
	return &types.DeploymentController{Type: types.DeploymentControllerType(cfg.Type)}
}

func toDeploymentConfiguration(cfg *ecscfg.DeploymentConfigurationConfig) *types.DeploymentConfiguration {
	if cfg == nil {
		return nil
	}
	out := &types.DeploymentConfiguration{}
	if cfg.MaximumPercent > 0 {
		out.MaximumPercent = aws.Int32(cfg.MaximumPercent)
	}
	if cfg.MinimumHealthyPercent > 0 {
		out.MinimumHealthyPercent = aws.Int32(cfg.MinimumHealthyPercent)
	}
	if cfg.DeploymentCircuitBreaker != nil {
		out.DeploymentCircuitBreaker = &types.DeploymentCircuitBreaker{Enable: cfg.DeploymentCircuitBreaker.Enable, Rollback: cfg.DeploymentCircuitBreaker.Rollback}
	}
	if len(cfg.Alarms) > 0 {
		alarms := &types.DeploymentAlarms{}
		for _, item := range cfg.Alarms {
			alarms.AlarmNames = append(alarms.AlarmNames, item.Name)
			alarms.Enable = item.Enable
			alarms.Rollback = item.Rollback
		}
		out.Alarms = alarms
	}
	if cfg.Strategy != "" {
		out.Strategy = types.DeploymentStrategy(cfg.Strategy)
	}
	if cfg.BakeTimeInMinutes > 0 {
		out.BakeTimeInMinutes = aws.Int32(cfg.BakeTimeInMinutes)
	}
	return out
}

func toLoadBalancers(items []ecscfg.LoadBalancerConfig) []types.LoadBalancer {
	out := make([]types.LoadBalancer, 0, len(items))
	for _, item := range items {
		out = append(out, types.LoadBalancer{TargetGroupArn: aws.String(item.TargetGroupARN), ContainerName: aws.String(item.ContainerName), ContainerPort: aws.Int32(item.ContainerPort)})
	}
	return out
}

func toServiceRegistries(items []ecscfg.ServiceRegistryConfig) []types.ServiceRegistry {
	out := make([]types.ServiceRegistry, 0, len(items))
	for _, item := range items {
		v := types.ServiceRegistry{RegistryArn: aws.String(item.RegistryARN)}
		if item.Port > 0 {
			v.Port = aws.Int32(item.Port)
		}
		if item.ContainerName != "" {
			v.ContainerName = aws.String(item.ContainerName)
		}
		if item.ContainerPort > 0 {
			v.ContainerPort = aws.Int32(item.ContainerPort)
		}
		out = append(out, v)
	}
	return out
}

func toPlacementConstraints(items []ecscfg.PlacementConstraintConfig) []types.PlacementConstraint {
	out := make([]types.PlacementConstraint, 0, len(items))
	for _, item := range items {
		v := types.PlacementConstraint{Type: types.PlacementConstraintType(item.Type)}
		if item.Expression != "" {
			v.Expression = aws.String(item.Expression)
		}
		out = append(out, v)
	}
	return out
}

func toPlacementStrategies(items []ecscfg.PlacementStrategyConfig) []types.PlacementStrategy {
	out := make([]types.PlacementStrategy, 0, len(items))
	for _, item := range items {
		v := types.PlacementStrategy{Type: types.PlacementStrategyType(item.Type)}
		if item.Field != "" {
			v.Field = aws.String(item.Field)
		}
		out = append(out, v)
	}
	return out
}

func toServiceConnect(cfg *ecscfg.ServiceConnectConfig) *types.ServiceConnectConfiguration {
	out := &types.ServiceConnectConfiguration{Enabled: cfg.Enabled}
	if cfg.Namespace != "" {
		out.Namespace = aws.String(cfg.Namespace)
	}
	for _, item := range cfg.Services {
		svc := types.ServiceConnectService{PortName: aws.String(item.PortName)}
		if item.DiscoveryName != "" {
			svc.DiscoveryName = aws.String(item.DiscoveryName)
		}
		if item.IngressPortOverride > 0 {
			svc.IngressPortOverride = aws.Int32(item.IngressPortOverride)
		}
		for _, alias := range item.ClientAliases {
			svc.ClientAliases = append(svc.ClientAliases, types.ServiceConnectClientAlias{DnsName: aws.String(alias.DNSName), Port: aws.Int32(alias.Port)})
		}
		out.Services = append(out.Services, svc)
	}
	return out
}

// DeleteService drains and deletes an ECS service.
func (c *Client) DeleteService(ctx context.Context, clusterName, serviceName string) error {
	// Scale to 0 first so tasks are drained cleanly
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(clusterName),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(0),
	})
	if err != nil {
		return fmt.Errorf("draining service: %w", err)
	}

	_, err = c.ecs.DeleteService(ctx, &ecs.DeleteServiceInput{
		Cluster: aws.String(clusterName),
		Service: aws.String(serviceName),
		Force:   aws.Bool(true),
	})
	return err
}

// ScaleService updates the desired count of an ECS service.
func (c *Client) ScaleService(ctx context.Context, clusterName, serviceName string, desired int32) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(clusterName),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(desired),
	})
	return err
}

// DeployService registers a new task definition revision with the updated image
// and updates the service to use it. Returns the new task definition ARN.
func (c *Client) DeployService(ctx context.Context, clusterName, serviceName, containerName, image string) (string, error) {
	// 1. Describe the current service to get the task definition ARN
	svcOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil || len(svcOut.Services) == 0 {
		return "", fmt.Errorf("describing service: %w", err)
	}

	currentTaskDef := aws.ToString(svcOut.Services[0].TaskDefinition)

	// 2. Describe the task definition
	tdOut, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(currentTaskDef),
		Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
	})
	if err != nil {
		return "", fmt.Errorf("describing task definition: %w", err)
	}

	td := tdOut.TaskDefinition
	containers := td.ContainerDefinitions

	// 3. Update the target container's image
	updated := false
	for i := range containers {
		if containerName == "" || aws.ToString(containers[i].Name) == containerName {
			containers[i].Image = aws.String(image)
			updated = true
			break
		}
	}
	if !updated {
		return "", fmt.Errorf("container %q not found in task definition", containerName)
	}

	// 4. Register a new task definition revision
	newTD, err := c.ecs.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:                  td.Family,
		ContainerDefinitions:    containers,
		Cpu:                     td.Cpu,
		EnableFaultInjection:    td.EnableFaultInjection,
		EphemeralStorage:        td.EphemeralStorage,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		IpcMode:                 td.IpcMode,
		PidMode:                 td.PidMode,
		RequiresCompatibilities: td.RequiresCompatibilities,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		TaskRoleArn:             td.TaskRoleArn,
		Volumes:                 td.Volumes,
		PlacementConstraints:    td.PlacementConstraints,
		ProxyConfiguration:      td.ProxyConfiguration,
		RuntimePlatform:         td.RuntimePlatform,
		Tags:                    tdOut.Tags,
	})
	if err != nil {
		return "", fmt.Errorf("registering task definition: %w", err)
	}

	newTaskDefARN := aws.ToString(newTD.TaskDefinition.TaskDefinitionArn)

	// 5. Update the service to use the new task definition
	_, err = c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:        aws.String(clusterName),
		Service:        aws.String(serviceName),
		TaskDefinition: aws.String(newTaskDefARN),
	})
	if err != nil {
		return "", err
	}
	return newTaskDefARN, nil
}

// WaitForDeployment polls the service deployments until the deployment using
// newTaskDefARN reaches a PRIMARY/COMPLETED state with runningCount == desiredCount,
// printing progress dots every 5 seconds. Times out after 10 minutes.
func (c *Client) WaitForDeployment(ctx context.Context, clusterName, serviceName, newTaskDefARN string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for deployment to complete (10m)")
		case <-ticker.C:
			svcOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: []string{serviceName},
			})
			if err != nil || len(svcOut.Services) == 0 {
				return fmt.Errorf("describing service: %w", err)
			}

			svc := svcOut.Services[0]

			// Find the deployment for our new task definition
			for _, d := range svc.Deployments {
				if aws.ToString(d.TaskDefinition) != newTaskDefARN {
					continue
				}

				fmt.Printf("  [%s] desired=%d running=%d pending=%d\n",
					string(d.RolloutState),
					d.DesiredCount,
					d.RunningCount,
					d.PendingCount,
				)

				switch d.RolloutState {
				case types.DeploymentRolloutStateCompleted:
					return nil
				case types.DeploymentRolloutStateFailed:
					reason := aws.ToString(d.RolloutStateReason)
					return fmt.Errorf("deployment failed: %s", reason)
				}
			}
		}
	}
}

// WaitForServiceStable polls until a service has runningCount == desiredCount
// and no pending tasks. Times out after 10 minutes.
func (c *Client) WaitForServiceStable(ctx context.Context, clusterName, serviceName string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*60*time.Second)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for service to stabilise (10m)")
		case <-ticker.C:
			out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: []string{serviceName},
			})
			if err != nil || len(out.Services) == 0 {
				continue
			}
			svc := out.Services[0]
			fmt.Printf("  desired=%d running=%d pending=%d\n",
				svc.DesiredCount, svc.RunningCount, svc.PendingCount)
			if svc.RunningCount == svc.DesiredCount && svc.PendingCount == 0 {
				return nil
			}
		}
	}
}

// FetchLogs retrieves CloudWatch logs for a service. If opts.Tail is true,
// it streams continuously (--follow), polling every 2 seconds for new events.
func (c *Client) FetchLogs(ctx context.Context, opts LogsOptions, tail int) error {
	// Resolve log group and stream prefix from the task definition
	svcOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(opts.Cluster),
		Services: []string{opts.Service},
	})
	if err != nil || len(svcOut.Services) == 0 {
		return fmt.Errorf("describing service: %w", err)
	}

	tdOut, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: svcOut.Services[0].TaskDefinition,
	})
	if err != nil {
		return fmt.Errorf("describing task definition: %w", err)
	}

	containers := tdOut.TaskDefinition.ContainerDefinitions
	if len(containers) == 0 {
		return fmt.Errorf("no containers found in task definition")
	}

	// Pick the target container
	container := containers[0]
	for _, cont := range containers {
		if opts.ContainerName != "" && aws.ToString(cont.Name) == opts.ContainerName {
			container = cont
			break
		}
	}

	if container.LogConfiguration == nil {
		return fmt.Errorf("container %q has no log configuration", aws.ToString(container.Name))
	}

	logGroup := container.LogConfiguration.Options["awslogs-group"]
	streamPrefix := container.LogConfiguration.Options["awslogs-stream-prefix"]
	containerName := aws.ToString(container.Name)

	// Find a running task to get the task ID for the stream name
	listOut, err := c.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     aws.String(opts.Cluster),
		ServiceName: aws.String(opts.Service),
	})
	if err != nil || len(listOut.TaskArns) == 0 {
		return fmt.Errorf("no running tasks found for service %q", opts.Service)
	}
	// ECS awslogs streams are named <prefix>/<container>/<task-id>. Read every
	// task in the service so a scaled service does not silently hide logs.
	logStreams := make([]string, 0, len(listOut.TaskArns))
	for _, taskARN := range listOut.TaskArns {
		taskID := taskARN
		if parts := strings.Split(taskARN, "/"); len(parts) > 1 {
			taskID = parts[len(parts)-1]
		}
		logStreams = append(logStreams, fmt.Sprintf("%s/%s/%s", streamPrefix, containerName, taskID))
	}

	if !opts.Tail {
		// One-shot fetch
		for _, stream := range logStreams {
			out, err := c.logs.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
				LogGroupName:  aws.String(logGroup),
				LogStreamName: aws.String(stream),
				Limit:         aws.Int32(int32(tail)),
				StartFromHead: aws.Bool(false),
			})
			if err != nil {
				return fmt.Errorf("fetching logs from %s: %w", stream, err)
			}
			for _, event := range out.Events {
				fmt.Printf("%s %s\n", time.UnixMilli(aws.ToInt64(event.Timestamp)).Format(time.RFC3339), aws.ToString(event.Message))
			}
		}
		return nil
	}

	// --follow: stream continuously using nextForwardToken
	fmt.Fprintf(os.Stderr, "Streaming logs for %s/%s (Ctrl+C to stop)...\n", opts.Service, containerName)
	nextTokens := make(map[string]*string, len(logStreams))
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, stream := range logStreams {
				input := &cloudwatchlogs.GetLogEventsInput{LogGroupName: aws.String(logGroup), LogStreamName: aws.String(stream), StartFromHead: aws.Bool(false)}
				if nextTokens[stream] != nil {
					input.NextToken = nextTokens[stream]
				} else {
					input.Limit = aws.Int32(int32(tail))
				}
				out, err := c.logs.GetLogEvents(ctx, input)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", stream, err)
					continue
				}
				for _, event := range out.Events {
					fmt.Printf("%s %s\n", time.UnixMilli(aws.ToInt64(event.Timestamp)).Format(time.RFC3339), aws.ToString(event.Message))
				}
				if out.NextForwardToken != nil && (nextTokens[stream] == nil || *out.NextForwardToken != *nextTokens[stream]) {
					nextTokens[stream] = out.NextForwardToken
				}
			}
		}
	}
}

// ExecInTask opens an interactive shell session in a running ECS task.
// Requires ECS Exec to be enabled on the service and task.
func (c *Client) ExecInTask(ctx context.Context, opts ExecOptions) error {
	taskID := opts.TaskID

	// If no task ID provided, find a running task for the service
	if taskID == "" {
		listOut, err := c.ecs.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:     aws.String(opts.Cluster),
			ServiceName: aws.String(opts.Service),
		})
		if err != nil || len(listOut.TaskArns) == 0 {
			return fmt.Errorf("no running tasks found for service %q", opts.Service)
		}
		taskID = listOut.TaskArns[0]
	}

	container := opts.Container
	if container == "" {
		// Resolve container name from task definition
		tasksOut, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(opts.Cluster),
			Tasks:   []string{taskID},
		})
		if err != nil || len(tasksOut.Tasks) == 0 {
			return fmt.Errorf("describing task: %w", err)
		}
		if len(tasksOut.Tasks[0].Containers) > 0 {
			container = aws.ToString(tasksOut.Tasks[0].Containers[0].Name)
		}
	}

	out, err := c.ecs.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(opts.Cluster),
		Task:        aws.String(taskID),
		Container:   aws.String(container),
		Command:     aws.String(opts.Command),
		Interactive: true,
	})
	if err != nil {
		return fmt.Errorf("executing command: %w", err)
	}

	// The session token is used with the SSM session manager plugin
	fmt.Printf("Session started: %s\n", aws.ToString(out.Session.SessionId))
	fmt.Println("Note: pipe this through the AWS SSM session-manager-plugin for full interactivity.")
	return nil
}

// ListClusters lists all ECS clusters in the account/region and prints them.
func (c *Client) ListClusters(ctx context.Context) error {
	var nextToken *string
	fmt.Printf("%-40s %-10s %s\n", "NAME", "STATUS", "ARN")
	fmt.Println("--------------------------------------------------------------------------------")

	for {
		listOut, err := c.ecs.ListClusters(ctx, &ecs.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("listing clusters: %w", err)
		}
		if len(listOut.ClusterArns) == 0 {
			fmt.Println("No clusters found.")
			return nil
		}

		descOut, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: listOut.ClusterArns,
		})
		if err != nil {
			return fmt.Errorf("describing clusters: %w", err)
		}

		for _, cl := range descOut.Clusters {
			status := ""
			if cl.Status != nil {
				status = *cl.Status
			}
			fmt.Printf("%-40s %-10s %s\n",
				aws.ToString(cl.ClusterName),
				status,
				aws.ToString(cl.ClusterArn),
			)
		}

		if listOut.NextToken == nil {
			break
		}
		nextToken = listOut.NextToken
	}
	return nil
}

// ListServices lists all services in a given ECS cluster and prints them.
func (c *Client) ListServices(ctx context.Context, clusterName string) error {
	var nextToken *string
	fmt.Printf("%-40s %-10s %-8s %-8s %s\n", "NAME", "STATUS", "DESIRED", "RUNNING", "TASK DEFINITION")
	fmt.Println("--------------------------------------------------------------------------------")

	for {
		listOut, err := c.ecs.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:   aws.String(clusterName),
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("listing services: %w", err)
		}
		if len(listOut.ServiceArns) == 0 {
			fmt.Println("No services found.")
			return nil
		}

		descOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: listOut.ServiceArns,
		})
		if err != nil {
			return fmt.Errorf("describing services: %w", err)
		}

		for _, svc := range descOut.Services {
			fmt.Printf("%-40s %-10s %-8d %-8d %s\n",
				aws.ToString(svc.ServiceName),
				aws.ToString(svc.Status),
				svc.DesiredCount,
				svc.RunningCount,
				aws.ToString(svc.TaskDefinition),
			)
		}

		if listOut.NextToken == nil {
			break
		}
		nextToken = listOut.NextToken
	}
	return nil
}

// PrintClusterDetail prints a detailed description of an ECS cluster.
func (c *Client) PrintClusterDetail(ctx context.Context, clusterName string) error {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
		Include:  []types.ClusterField{types.ClusterFieldTags, types.ClusterFieldStatistics},
	})
	if err != nil {
		return fmt.Errorf("describing cluster: %w", err)
	}
	if len(out.Clusters) == 0 {
		return fmt.Errorf("cluster %q not found", clusterName)
	}

	cl := out.Clusters[0]

	fmt.Printf("Name:       %s\n", aws.ToString(cl.ClusterName))
	fmt.Printf("ARN:        %s\n", aws.ToString(cl.ClusterArn))
	fmt.Printf("Status:     %s\n", aws.ToString(cl.Status))
	fmt.Printf("\nTask Counts:\n")
	fmt.Printf("  Running:    %d\n", cl.RunningTasksCount)
	fmt.Printf("  Pending:    %d\n", cl.PendingTasksCount)
	fmt.Printf("  Active Services: %d\n", cl.ActiveServicesCount)
	fmt.Printf("  Registered Instances: %d\n", cl.RegisteredContainerInstancesCount)

	if len(cl.CapacityProviders) > 0 {
		fmt.Printf("\nCapacity Providers:\n")
		for _, cp := range cl.CapacityProviders {
			fmt.Printf("  - %s\n", cp)
		}
	}

	if len(cl.Tags) > 0 {
		fmt.Printf("\nTags:\n")
		for _, t := range cl.Tags {
			fmt.Printf("  %s = %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
		}
	}

	return nil
}

// PrintServiceDetail prints a detailed description of an ECS service.
func (c *Client) PrintServiceDetail(ctx context.Context, clusterName, serviceName string) error {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil {
		return fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return fmt.Errorf("service %q not found in cluster %q", serviceName, clusterName)
	}

	svc := out.Services[0]

	fmt.Printf("Name:           %s\n", aws.ToString(svc.ServiceName))
	fmt.Printf("ARN:            %s\n", aws.ToString(svc.ServiceArn))
	fmt.Printf("Cluster:        %s\n", aws.ToString(svc.ClusterArn))
	fmt.Printf("Status:         %s\n", aws.ToString(svc.Status))
	fmt.Printf("Launch Type:    %s\n", string(svc.LaunchType))
	fmt.Printf("Task Definition:%s\n", aws.ToString(svc.TaskDefinition))
	fmt.Printf("\nTask Counts:\n")
	fmt.Printf("  Desired:  %d\n", svc.DesiredCount)
	fmt.Printf("  Running:  %d\n", svc.RunningCount)
	fmt.Printf("  Pending:  %d\n", svc.PendingCount)

	if svc.NetworkConfiguration != nil && svc.NetworkConfiguration.AwsvpcConfiguration != nil {
		vpc := svc.NetworkConfiguration.AwsvpcConfiguration
		fmt.Printf("\nNetwork:\n")
		fmt.Printf("  Subnets:        %v\n", vpc.Subnets)
		fmt.Printf("  Security Groups:%v\n", vpc.SecurityGroups)
		fmt.Printf("  Public IP:      %s\n", string(vpc.AssignPublicIp))
	}

	if len(svc.Deployments) > 0 {
		fmt.Printf("\nDeployments:\n")
		for _, d := range svc.Deployments {
			fmt.Printf("  [%s] %s  desired=%d running=%d pending=%d\n",
				aws.ToString(d.Status),
				string(d.RolloutState),
				d.DesiredCount,
				d.RunningCount,
				d.PendingCount,
			)
		}
	}

	if len(svc.Events) > 0 {
		fmt.Printf("\nRecent Events:\n")
		limit := 5
		if len(svc.Events) < limit {
			limit = len(svc.Events)
		}
		for _, e := range svc.Events[:limit] {
			ts := ""
			if e.CreatedAt != nil {
				ts = e.CreatedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("  %s  %s\n", ts, aws.ToString(e.Message))
		}
	}

	if len(svc.Tags) > 0 {
		fmt.Printf("\nTags:\n")
		for _, t := range svc.Tags {
			fmt.Printf("  %s = %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
		}
	}

	return nil
}

// ClusterExists returns true if an ACTIVE cluster with the given name exists.
func (c *Client) ClusterExists(ctx context.Context, clusterName string) (bool, error) {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
	})
	if err != nil {
		return false, fmt.Errorf("describing cluster: %w", err)
	}
	for _, cl := range out.Clusters {
		if aws.ToString(cl.ClusterName) == clusterName && aws.ToString(cl.Status) == "ACTIVE" {
			return true, nil
		}
	}
	return false, nil
}

// ServiceExists returns true if an ACTIVE service with the given name exists in the cluster.
func (c *Client) ServiceExists(ctx context.Context, clusterName, serviceName string) (bool, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil {
		return false, fmt.Errorf("describing service: %w", err)
	}
	for _, svc := range out.Services {
		if aws.ToString(svc.ServiceName) == serviceName && aws.ToString(svc.Status) == "ACTIVE" {
			return true, nil
		}
	}
	return false, nil
}

// ReconcileService compares a desired ServiceConfig against the live service
// and applies changes where there is drift. Returns true if any changes were made.
func (c *Client) ReconcileService(ctx context.Context, cfg *ecscfg.ServiceConfig, dryRun bool) (bool, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Cluster),
		Services: []string{cfg.Name},
		Include:  []types.ServiceField{types.ServiceFieldTags},
	})
	if err != nil {
		return false, fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return false, fmt.Errorf("service %q not found in cluster %q", cfg.Name, cfg.Cluster)
	}

	svc := out.Services[0]
	changed := false
	changes := make([]string, 0)
	updateInput := &ecs.UpdateServiceInput{
		Cluster: aws.String(cfg.Cluster),
		Service: aws.String(cfg.Name),
	}
	currentScheduling := string(svc.SchedulingStrategy)
	if currentScheduling == "" {
		currentScheduling = "REPLICA"
	}
	desiredScheduling := cfg.SchedulingStrategy
	if desiredScheduling == "" {
		desiredScheduling = "REPLICA"
	}
	if currentScheduling != desiredScheduling {
		return false, fmt.Errorf("service schedulingStrategy cannot be changed from %s to %s; replace the service", currentScheduling, desiredScheduling)
	}
	desiredLaunchType := cfg.LaunchType
	if desiredLaunchType == "" {
		desiredLaunchType = "FARGATE"
	}
	if cfg.CapacityProviderStrategy == nil && (cfg.LaunchTypeConfigured || cfg.LaunchType != "") && string(svc.LaunchType) != desiredLaunchType {
		return false, fmt.Errorf("service launchType cannot be changed from %q to %q in place; replace the service or configure capacityProviderStrategy", svc.LaunchType, desiredLaunchType)
	}

	if desiredScheduling == "REPLICA" && cfg.AutoScaling == nil && (cfg.DesiredCountConfigured || cfg.DesiredCount != 0) && svc.DesiredCount != cfg.DesiredCount {
		fmt.Printf("  desiredCount: %d → %d\n", svc.DesiredCount, cfg.DesiredCount)
		updateInput.DesiredCount = aws.Int32(cfg.DesiredCount)
		changed = true
		changes = append(changes, "desiredCount")
	}
	if taskDefinitionRef(aws.ToString(svc.TaskDefinition)) != taskDefinitionRef(cfg.TaskDefinition) {
		fmt.Printf("  taskDefinition: %s → %s\n", aws.ToString(svc.TaskDefinition), cfg.TaskDefinition)
		updateInput.TaskDefinition = aws.String(cfg.TaskDefinition)
		changed = true
		changes = append(changes, "taskDefinition")
	}
	if cfg.DeploymentController != nil && !reflect.DeepEqual(svc.DeploymentController, toDeploymentController(cfg.DeploymentController)) {
		updateInput.DeploymentController = toDeploymentController(cfg.DeploymentController)
		changed = true
		changes = append(changes, "deploymentController")
	}
	if cfg.DeploymentConfiguration != nil && !reflect.DeepEqual(svc.DeploymentConfiguration, toDeploymentConfiguration(cfg.DeploymentConfiguration)) {
		updateInput.DeploymentConfiguration = toDeploymentConfiguration(cfg.DeploymentConfiguration)
		changed = true
		changes = append(changes, "deploymentConfiguration")
	}
	if cfg.CapacityProviderStrategy != nil && !sameSlice(svc.CapacityProviderStrategy, toCapacityProviderStrategy(cfg.CapacityProviderStrategy)) {
		updateInput.CapacityProviderStrategy = toCapacityProviderStrategy(cfg.CapacityProviderStrategy)
		changed = true
		changes = append(changes, "capacityProviderStrategy")
	}
	if cfg.NetworkConfig != nil {
		want := &types.NetworkConfiguration{AwsvpcConfiguration: &types.AwsVpcConfiguration{Subnets: cfg.NetworkConfig.Subnets, SecurityGroups: cfg.NetworkConfig.SecurityGroups, AssignPublicIp: types.AssignPublicIp(cfg.NetworkConfig.AssignPublicIP)}}
		if !reflect.DeepEqual(svc.NetworkConfiguration, want) {
			updateInput.NetworkConfiguration = want
			changed = true
			changes = append(changes, "network")
		}
	}
	if cfg.LoadBalancers != nil && !sameSlice(svc.LoadBalancers, toLoadBalancers(cfg.LoadBalancers)) {
		fmt.Printf("  loadBalancers: updating %d item(s)\n", len(cfg.LoadBalancers))
		updateInput.LoadBalancers = toLoadBalancers(cfg.LoadBalancers)
		changed = true
		changes = append(changes, "loadBalancers")
	}
	if cfg.ServiceRegistries != nil && !sameSlice(svc.ServiceRegistries, toServiceRegistries(cfg.ServiceRegistries)) {
		fmt.Printf("  serviceRegistries: updating %d item(s)\n", len(cfg.ServiceRegistries))
		updateInput.ServiceRegistries = toServiceRegistries(cfg.ServiceRegistries)
		changed = true
		changes = append(changes, "serviceRegistries")
	}
	if cfg.PlacementConstraints != nil && !sameSlice(svc.PlacementConstraints, toPlacementConstraints(cfg.PlacementConstraints)) {
		fmt.Printf("  placementConstraints: updating %d item(s)\n", len(cfg.PlacementConstraints))
		updateInput.PlacementConstraints = toPlacementConstraints(cfg.PlacementConstraints)
		changed = true
		changes = append(changes, "placementConstraints")
	}
	if cfg.PlacementStrategy != nil && !sameSlice(svc.PlacementStrategy, toPlacementStrategies(cfg.PlacementStrategy)) {
		fmt.Printf("  placementStrategy: updating %d item(s)\n", len(cfg.PlacementStrategy))
		updateInput.PlacementStrategy = toPlacementStrategies(cfg.PlacementStrategy)
		changed = true
		changes = append(changes, "placementStrategy")
	}
	if cfg.ServiceConnect != nil {
		want := toServiceConnect(cfg.ServiceConnect)
		var current *types.ServiceConnectConfiguration
		for _, deployment := range svc.Deployments {
			if aws.ToString(deployment.Status) == "PRIMARY" {
				current = deployment.ServiceConnectConfiguration
				break
			}
		}
		if !reflect.DeepEqual(current, want) {
			updateInput.ServiceConnectConfiguration = want
			changed = true
			changes = append(changes, "serviceConnect")
		}
	}
	if cfg.EnableExecuteCommand != nil && svc.EnableExecuteCommand != *cfg.EnableExecuteCommand {
		updateInput.EnableExecuteCommand = cfg.EnableExecuteCommand
		changed = true
		changes = append(changes, "enableExecuteCommand")
	}
	if (cfg.HealthCheckGraceConfigured || cfg.HealthCheckGracePeriodSeconds != 0) && aws.ToInt32(svc.HealthCheckGracePeriodSeconds) != cfg.HealthCheckGracePeriodSeconds {
		updateInput.HealthCheckGracePeriodSeconds = aws.Int32(cfg.HealthCheckGracePeriodSeconds)
		changed = true
		changes = append(changes, "healthCheckGracePeriodSeconds")
	}
	if cfg.PlatformVersion != "" && aws.ToString(svc.PlatformVersion) != cfg.PlatformVersion {
		updateInput.PlatformVersion = aws.String(cfg.PlatformVersion)
		changed = true
		changes = append(changes, "platformVersion")
	}
	if cfg.PropagateTags != "" && string(svc.PropagateTags) != cfg.PropagateTags {
		updateInput.PropagateTags = types.PropagateTags(cfg.PropagateTags)
		changed = true
		changes = append(changes, "propagateTags")
	}
	if cfg.EnableECSManagedTags != nil && *cfg.EnableECSManagedTags != svc.EnableECSManagedTags {
		updateInput.EnableECSManagedTags = cfg.EnableECSManagedTags
		changed = true
		changes = append(changes, "enableECSManagedTags")
	}
	if cfg.Tags != nil && !tagsEqual(svc.Tags, cfg.Tags) {
		changed = true
		changes = append(changes, "tags")
	}
	autoScalingDrift := false
	if cfg.AutoScaling != nil {
		var err error
		autoScalingDrift, err = c.serviceAutoScalingDrift(ctx, cfg)
		if err != nil {
			return false, err
		}
		changed = changed || autoScalingDrift
		if autoScalingDrift {
			changes = append(changes, "autoScaling")
		}
	}

	if changed {
		if dryRun {
			fmt.Printf("[dry-run] Service %q would update: %s.\n", cfg.Name, strings.Join(changes, ", "))
		} else {
			if updateInput.DesiredCount != nil || updateInput.TaskDefinition != nil || updateInput.DeploymentController != nil || updateInput.DeploymentConfiguration != nil || updateInput.CapacityProviderStrategy != nil || updateInput.NetworkConfiguration != nil || updateInput.LoadBalancers != nil || updateInput.ServiceRegistries != nil || updateInput.PlacementConstraints != nil || updateInput.PlacementStrategy != nil || updateInput.ServiceConnectConfiguration != nil || updateInput.EnableExecuteCommand != nil || updateInput.HealthCheckGracePeriodSeconds != nil || updateInput.PlatformVersion != nil || updateInput.PropagateTags != "" || updateInput.EnableECSManagedTags != nil {
				if _, err := c.ecs.UpdateService(ctx, updateInput); err != nil {
					return false, fmt.Errorf("updating service: %w", err)
				}
			}
			if cfg.Tags != nil && !tagsEqual(svc.Tags, cfg.Tags) {
				if err := c.reconcileTags(ctx, aws.ToString(svc.ServiceArn), svc.Tags, cfg.Tags); err != nil {
					return false, fmt.Errorf("reconciling service tags: %w", err)
				}
			}
			if autoScalingDrift {
				if err := c.configureServiceAutoScaling(ctx, cfg); err != nil {
					return false, fmt.Errorf("configuring service auto scaling: %w", err)
				}
			}
			fmt.Printf("✓ Service %q updated.\n", cfg.Name)
		}
	}

	return changed, nil
}

func taskDefinitionRef(value string) string {
	if slash := strings.LastIndex(value, "/"); slash >= 0 {
		return value[slash+1:]
	}
	return value
}

func sameSlice[T any](current, desired []T) bool {
	if len(current) == 0 && len(desired) == 0 {
		return true
	}
	return reflect.DeepEqual(current, desired)
}

func tagsEqual(current []types.Tag, desired map[string]string) bool {
	if len(current) != len(desired) {
		return false
	}
	for _, tag := range current {
		if desired[aws.ToString(tag.Key)] != aws.ToString(tag.Value) {
			return false
		}
	}
	return true
}

func (c *Client) reconcileTags(ctx context.Context, arn string, current []types.Tag, desired map[string]string) error {
	var remove []string
	for _, tag := range current {
		if _, ok := desired[aws.ToString(tag.Key)]; !ok {
			remove = append(remove, aws.ToString(tag.Key))
		}
	}
	if len(remove) > 0 {
		if _, err := c.ecs.UntagResource(ctx, &ecs.UntagResourceInput{ResourceArn: aws.String(arn), TagKeys: remove}); err != nil {
			return err
		}
	}
	var add []types.Tag
	for key, value := range desired {
		found := false
		for _, tag := range current {
			if aws.ToString(tag.Key) == key && aws.ToString(tag.Value) == value {
				found = true
				break
			}
		}
		if !found {
			add = append(add, types.Tag{Key: aws.String(key), Value: aws.String(value)})
		}
	}
	if len(add) > 0 {
		_, err := c.ecs.TagResource(ctx, &ecs.TagResourceInput{ResourceArn: aws.String(arn), Tags: add})
		return err
	}
	return nil
}

// PrintTasks lists running tasks in a cluster, optionally filtered by service.
func (c *Client) PrintTasks(ctx context.Context, clusterName, serviceName string) error {
	input := &ecs.ListTasksInput{
		Cluster: aws.String(clusterName),
	}
	if serviceName != "" {
		input.ServiceName = aws.String(serviceName)
	}

	listOut, err := c.ecs.ListTasks(ctx, input)
	if err != nil {
		return fmt.Errorf("listing tasks: %w", err)
	}
	if len(listOut.TaskArns) == 0 {
		fmt.Println("No running tasks found.")
		return nil
	}

	descOut, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(clusterName),
		Tasks:   listOut.TaskArns,
	})
	if err != nil {
		return fmt.Errorf("describing tasks: %w", err)
	}

	fmt.Printf("%-36s %-12s %-12s %-30s %s\n", "TASK ID", "STATUS", "LAUNCH TYPE", "STARTED AT", "TASK DEFINITION")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────")

	for _, task := range descOut.Tasks {
		// Extract short task ID from the last segment of the ARN
		taskID := aws.ToString(task.TaskArn)
		if parts := strings.Split(taskID, "/"); len(parts) > 1 {
			taskID = parts[len(parts)-1]
		}

		startedAt := ""
		if task.StartedAt != nil {
			startedAt = task.StartedAt.Format("2006-01-02 15:04:05")
		}

		// Short task definition
		td := aws.ToString(task.TaskDefinitionArn)
		if idx := len(td) - 1; idx > 0 {
			for i := idx; i >= 0; i-- {
				if td[i] == '/' {
					td = td[i+1:]
					break
				}
			}
		}

		fmt.Printf("%-36s %-12s %-12s %-30s %s\n",
			taskID,
			aws.ToString(task.LastStatus),
			string(task.LaunchType),
			startedAt,
			td,
		)
	}
	return nil
}

// DescribeClusterResource fetches a cluster from AWS and returns it as a state.Resource.
func (c *Client) DescribeClusterResource(ctx context.Context, clusterName string) (*state.Resource, error) {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
		Include:  []types.ClusterField{types.ClusterFieldSettings, types.ClusterFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("describing cluster: %w", err)
	}
	for _, cl := range out.Clusters {
		if aws.ToString(cl.ClusterName) == clusterName && aws.ToString(cl.Status) == "ACTIVE" {
			cfg := &ecscfg.ClusterConfig{Name: clusterName, CapacityProviders: cl.CapacityProviders}
			for _, strategy := range cl.DefaultCapacityProviderStrategy {
				cfg.DefaultCapacityProviderStrategy = append(cfg.DefaultCapacityProviderStrategy, ecscfg.CapacityProviderStrategyConfig{CapacityProvider: aws.ToString(strategy.CapacityProvider), Weight: strategy.Weight, Base: strategy.Base})
			}
			if cl.ServiceConnectDefaults != nil {
				cfg.ServiceConnectDefaultsNamespace = aws.ToString(cl.ServiceConnectDefaults.Namespace)
			}
			cfg.Tags = make(map[string]string, len(cl.Tags))
			for _, tag := range cl.Tags {
				cfg.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
			}
			configuration, err := yaml.Marshal(cfg)
			if err != nil {
				return nil, fmt.Errorf("serializing imported cluster configuration: %w", err)
			}
			return &state.Resource{
				Type:          state.ResourceTypeCluster,
				Name:          clusterName,
				ARN:           aws.ToString(cl.ClusterArn),
				CreatedBy:     "imported",
				Configuration: string(configuration),
			}, nil
		}
	}
	return nil, fmt.Errorf("cluster %q not found or not ACTIVE", clusterName)
}

// DescribeServiceResource fetches a service from AWS and returns it as a state.Resource.
func (c *Client) DescribeServiceResource(ctx context.Context, clusterName, serviceName string) (*state.Resource, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
		Include:  []types.ServiceField{types.ServiceFieldTags},
	})
	if err != nil {
		return nil, fmt.Errorf("describing service: %w", err)
	}
	for _, svc := range out.Services {
		if aws.ToString(svc.ServiceName) == serviceName && aws.ToString(svc.Status) == "ACTIVE" {
			cfg := serviceConfigFromAWS(svc)
			if c.autoscaling != nil {
				if scaling, err := c.serviceAutoScalingConfig(ctx, clusterName, serviceName); err != nil {
					return nil, err
				} else if scaling != nil {
					cfg.AutoScaling = scaling
				}
			}
			configuration, err := yaml.Marshal(cfg)
			if err != nil {
				return nil, fmt.Errorf("serializing imported service configuration: %w", err)
			}
			return &state.Resource{
				Type:          state.ResourceTypeService,
				Name:          serviceName,
				ARN:           aws.ToString(svc.ServiceArn),
				Cluster:       clusterName,
				CreatedBy:     "imported",
				Configuration: string(configuration),
			}, nil
		}
	}
	return nil, fmt.Errorf("service %q not found or not ACTIVE in cluster %q", serviceName, clusterName)
}

func serviceConfigFromAWS(service types.Service) *ecscfg.ServiceConfig {
	scheduling := string(service.SchedulingStrategy)
	if scheduling == "" {
		scheduling = "REPLICA"
	}
	cfg := &ecscfg.ServiceConfig{
		Name: aws.ToString(service.ServiceName), Cluster: clusterNameFromARN(aws.ToString(service.ClusterArn)),
		TaskDefinition: aws.ToString(service.TaskDefinition), LaunchType: string(service.LaunchType), SchedulingStrategy: scheduling,
		DesiredCount: service.DesiredCount, HealthCheckGracePeriodSeconds: aws.ToInt32(service.HealthCheckGracePeriodSeconds),
		DesiredCountConfigured: true, HealthCheckGraceConfigured: true, LaunchTypeConfigured: service.LaunchType != "",
		EnableExecuteCommand: aws.Bool(service.EnableExecuteCommand), EnableECSManagedTags: aws.Bool(service.EnableECSManagedTags),
		PropagateTags: string(service.PropagateTags), PlatformVersion: aws.ToString(service.PlatformVersion),
	}
	if scheduling == "DAEMON" {
		cfg.DesiredCount = 0
	}
	for _, strategy := range service.CapacityProviderStrategy {
		cfg.CapacityProviderStrategy = append(cfg.CapacityProviderStrategy, ecscfg.CapacityProviderStrategyConfig{CapacityProvider: aws.ToString(strategy.CapacityProvider), Weight: strategy.Weight, Base: strategy.Base})
	}
	if service.NetworkConfiguration != nil && service.NetworkConfiguration.AwsvpcConfiguration != nil {
		network := service.NetworkConfiguration.AwsvpcConfiguration
		cfg.NetworkConfig = &ecscfg.NetworkConfig{Subnets: network.Subnets, SecurityGroups: network.SecurityGroups, AssignPublicIP: string(network.AssignPublicIp)}
	}
	if service.DeploymentController != nil {
		cfg.DeploymentController = &ecscfg.DeploymentControllerConfig{Type: string(service.DeploymentController.Type)}
	}
	if deployment := service.DeploymentConfiguration; deployment != nil {
		cfg.DeploymentConfiguration = deploymentConfigFromAWS(deployment)
	}
	for _, item := range service.LoadBalancers {
		cfg.LoadBalancers = append(cfg.LoadBalancers, ecscfg.LoadBalancerConfig{TargetGroupARN: aws.ToString(item.TargetGroupArn), ContainerName: aws.ToString(item.ContainerName), ContainerPort: aws.ToInt32(item.ContainerPort)})
	}
	for _, item := range service.ServiceRegistries {
		cfg.ServiceRegistries = append(cfg.ServiceRegistries, ecscfg.ServiceRegistryConfig{RegistryARN: aws.ToString(item.RegistryArn), Port: aws.ToInt32(item.Port), ContainerName: aws.ToString(item.ContainerName), ContainerPort: aws.ToInt32(item.ContainerPort)})
	}
	for _, item := range service.PlacementConstraints {
		cfg.PlacementConstraints = append(cfg.PlacementConstraints, ecscfg.PlacementConstraintConfig{Type: string(item.Type), Expression: aws.ToString(item.Expression)})
	}
	for _, item := range service.PlacementStrategy {
		cfg.PlacementStrategy = append(cfg.PlacementStrategy, ecscfg.PlacementStrategyConfig{Type: string(item.Type), Field: aws.ToString(item.Field)})
	}
	for _, deployment := range service.Deployments {
		if aws.ToString(deployment.Status) == "PRIMARY" && deployment.ServiceConnectConfiguration != nil {
			cfg.ServiceConnect = serviceConnectConfigFromAWS(deployment.ServiceConnectConfiguration)
			break
		}
	}
	cfg.Tags = make(map[string]string, len(service.Tags))
	for _, tag := range service.Tags {
		cfg.Tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return cfg
}

func deploymentConfigFromAWS(value *types.DeploymentConfiguration) *ecscfg.DeploymentConfigurationConfig {
	if value == nil {
		return nil
	}
	cfg := &ecscfg.DeploymentConfigurationConfig{MaximumPercent: aws.ToInt32(value.MaximumPercent), MinimumHealthyPercent: aws.ToInt32(value.MinimumHealthyPercent), Strategy: string(value.Strategy), BakeTimeInMinutes: aws.ToInt32(value.BakeTimeInMinutes)}
	if value.DeploymentCircuitBreaker != nil {
		cfg.DeploymentCircuitBreaker = &ecscfg.DeploymentCircuitBreakerConfig{Enable: value.DeploymentCircuitBreaker.Enable, Rollback: value.DeploymentCircuitBreaker.Rollback}
	}
	if value.Alarms != nil {
		for _, name := range value.Alarms.AlarmNames {
			cfg.Alarms = append(cfg.Alarms, ecscfg.DeploymentAlarmConfig{Name: name, Enable: value.Alarms.Enable, Rollback: value.Alarms.Rollback})
		}
	}
	return cfg
}

func serviceConnectConfigFromAWS(value *types.ServiceConnectConfiguration) *ecscfg.ServiceConnectConfig {
	cfg := &ecscfg.ServiceConnectConfig{Enabled: value.Enabled, Namespace: aws.ToString(value.Namespace)}
	for _, service := range value.Services {
		item := ecscfg.ServiceConnectServiceConfig{PortName: aws.ToString(service.PortName), DiscoveryName: aws.ToString(service.DiscoveryName), IngressPortOverride: aws.ToInt32(service.IngressPortOverride)}
		for _, alias := range service.ClientAliases {
			item.ClientAliases = append(item.ClientAliases, ecscfg.ServiceConnectClientAliasConfig{DNSName: aws.ToString(alias.DnsName), Port: aws.ToInt32(alias.Port)})
		}
		cfg.Services = append(cfg.Services, item)
	}
	return cfg
}

func (c *Client) serviceAutoScalingConfig(ctx context.Context, cluster, name string) (*ecscfg.ServiceAutoScalingConfig, error) {
	resourceID := fmt.Sprintf("service/%s/%s", cluster, name)
	targets, err := c.autoscaling.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{ServiceNamespace: applicationautoscalingTypes.ServiceNamespaceEcs, ScalableDimension: applicationautoscalingTypes.ScalableDimensionECSServiceDesiredCount, ResourceIds: []string{resourceID}})
	if err != nil {
		return nil, fmt.Errorf("describing imported service auto scaling: %w", err)
	}
	if len(targets.ScalableTargets) == 0 {
		return nil, nil
	}
	cfg := &ecscfg.ServiceAutoScalingConfig{MinCapacity: aws.ToInt32(targets.ScalableTargets[0].MinCapacity), MaxCapacity: aws.ToInt32(targets.ScalableTargets[0].MaxCapacity)}
	policies, err := c.autoscaling.DescribeScalingPolicies(ctx, &applicationautoscaling.DescribeScalingPoliciesInput{ServiceNamespace: applicationautoscalingTypes.ServiceNamespaceEcs, ScalableDimension: applicationautoscalingTypes.ScalableDimensionECSServiceDesiredCount, ResourceId: aws.String(resourceID)})
	if err != nil {
		return nil, fmt.Errorf("describing imported service scaling policy: %w", err)
	}
	for _, policy := range policies.ScalingPolicies {
		if configuration := policy.TargetTrackingScalingPolicyConfiguration; configuration != nil {
			cfg.TargetValue = aws.ToFloat64(configuration.TargetValue)
			if configuration.PredefinedMetricSpecification != nil && configuration.PredefinedMetricSpecification.PredefinedMetricType == applicationautoscalingTypes.MetricTypeECSServiceAverageMemoryUtilization {
				cfg.Metric = "Memory"
			}
			break
		}
	}
	return cfg, nil
}

func clusterNameFromARN(value string) string {
	if index := strings.LastIndex(value, "/"); index >= 0 {
		return value[index+1:]
	}
	return value
}

// toECSTags converts a map of string tags to ECS Tag types.
func toECSTags(tags map[string]string) []types.Tag {
	result := make([]types.Tag, 0, len(tags))
	for k, v := range tags {
		k, v := k, v
		result = append(result, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return result
}
