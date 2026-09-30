package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

// mockECS implements ecsIface for testing.
// Panics on unexpected calls so tests fail loudly if untested code paths fire.
type mockECS struct {
	describeClustersOut *ecs.DescribeClustersOutput
	describeClustersErr error
	createClusterCalled bool
	createClusterErr    error
	createClusterInput  *ecs.CreateClusterInput
	describeServicesOut *ecs.DescribeServicesOutput
	describeServicesErr error
	createServiceCalled bool
	createServiceErr    error
	createServiceInput  *ecs.CreateServiceInput
	updateServiceInput  *ecs.UpdateServiceInput
	describeTaskDefOut  *ecs.DescribeTaskDefinitionOutput
	registerTaskDefIn   *ecs.RegisterTaskDefinitionInput
	registerTaskDefOut  *ecs.RegisterTaskDefinitionOutput
}

func (m *mockECS) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return m.describeClustersOut, m.describeClustersErr
}
func (m *mockECS) CreateCluster(_ context.Context, input *ecs.CreateClusterInput, _ ...func(*ecs.Options)) (*ecs.CreateClusterOutput, error) {
	m.createClusterCalled = true
	m.createClusterInput = input
	return &ecs.CreateClusterOutput{}, m.createClusterErr
}
func (m *mockECS) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return m.describeServicesOut, m.describeServicesErr
}
func (m *mockECS) CreateService(_ context.Context, input *ecs.CreateServiceInput, _ ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error) {
	m.createServiceCalled = true
	m.createServiceInput = input
	return &ecs.CreateServiceOutput{}, m.createServiceErr
}
func (m *mockECS) DeleteCluster(_ context.Context, _ *ecs.DeleteClusterInput, _ ...func(*ecs.Options)) (*ecs.DeleteClusterOutput, error) {
	panic("unexpected: DeleteCluster")
}
func (m *mockECS) DeleteService(_ context.Context, _ *ecs.DeleteServiceInput, _ ...func(*ecs.Options)) (*ecs.DeleteServiceOutput, error) {
	panic("unexpected: DeleteService")
}
func (m *mockECS) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	panic("unexpected: ListClusters")
}
func (m *mockECS) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	panic("unexpected: ListServices")
}
func (m *mockECS) UpdateService(_ context.Context, input *ecs.UpdateServiceInput, _ ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	m.updateServiceInput = input
	return &ecs.UpdateServiceOutput{}, nil
}
func (m *mockECS) DescribeTaskDefinition(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return m.describeTaskDefOut, nil
}
func (m *mockECS) RegisterTaskDefinition(_ context.Context, input *ecs.RegisterTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error) {
	m.registerTaskDefIn = input
	return m.registerTaskDefOut, nil
}
func (m *mockECS) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	panic("unexpected: DescribeTasks")
}
func (m *mockECS) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	panic("unexpected: ListTasks")
}
func (m *mockECS) ExecuteCommand(_ context.Context, _ *ecs.ExecuteCommandInput, _ ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error) {
	panic("unexpected: ExecuteCommand")
}
func (m *mockECS) CreateExpressGatewayService(_ context.Context, _ *ecs.CreateExpressGatewayServiceInput, _ ...func(*ecs.Options)) (*ecs.CreateExpressGatewayServiceOutput, error) {
	panic("unexpected: CreateExpressGatewayService")
}
func (m *mockECS) UpdateExpressGatewayService(_ context.Context, _ *ecs.UpdateExpressGatewayServiceInput, _ ...func(*ecs.Options)) (*ecs.UpdateExpressGatewayServiceOutput, error) {
	panic("unexpected: UpdateExpressGatewayService")
}
func (m *mockECS) DescribeExpressGatewayService(_ context.Context, _ *ecs.DescribeExpressGatewayServiceInput, _ ...func(*ecs.Options)) (*ecs.DescribeExpressGatewayServiceOutput, error) {
	panic("unexpected: DescribeExpressGatewayService")
}
func (m *mockECS) DeleteExpressGatewayService(_ context.Context, _ *ecs.DeleteExpressGatewayServiceInput, _ ...func(*ecs.Options)) (*ecs.DeleteExpressGatewayServiceOutput, error) {
	panic("unexpected: DeleteExpressGatewayService")
}
func (m *mockECS) RunTask(_ context.Context, _ *ecs.RunTaskInput, _ ...func(*ecs.Options)) (*ecs.RunTaskOutput, error) {
	panic("unexpected: RunTask")
}
func (m *mockECS) StopTask(_ context.Context, _ *ecs.StopTaskInput, _ ...func(*ecs.Options)) (*ecs.StopTaskOutput, error) {
	panic("unexpected: StopTask")
}
func (m *mockECS) TagResource(_ context.Context, _ *ecs.TagResourceInput, _ ...func(*ecs.Options)) (*ecs.TagResourceOutput, error) {
	panic("unexpected: TagResource")
}
func (m *mockECS) UntagResource(_ context.Context, _ *ecs.UntagResourceInput, _ ...func(*ecs.Options)) (*ecs.UntagResourceOutput, error) {
	panic("unexpected: UntagResource")
}

func testClient(mock *mockECS) *Client {
	return &Client{ecs: mock}
}

// --- CreateCluster existence check ---

func TestCreateCluster_AlreadyExists(t *testing.T) {
	active := "ACTIVE"
	mock := &mockECS{
		describeClustersOut: &ecs.DescribeClustersOutput{
			Clusters: []types.Cluster{
				{ClusterName: aws.String("my-cluster"), Status: &active},
			},
		},
	}
	if err := testClient(mock).CreateCluster(context.Background(), &ecscfg.ClusterConfig{Name: "my-cluster"}); err == nil {
		t.Fatal("expected error for existing cluster, got nil")
	}
	if mock.createClusterCalled {
		t.Error("SDK CreateCluster should not be called when cluster already exists")
	}
}

func TestCreateCluster_DoesNotExist(t *testing.T) {
	mock := &mockECS{
		describeClustersOut: &ecs.DescribeClustersOutput{Clusters: []types.Cluster{}},
	}
	if err := testClient(mock).CreateCluster(context.Background(), &ecscfg.ClusterConfig{Name: "new-cluster"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.createClusterCalled {
		t.Error("SDK CreateCluster should be called for a new cluster")
	}
}

func TestCreateCluster_DescribeError(t *testing.T) {
	mock := &mockECS{describeClustersErr: errors.New("network error")}
	if err := testClient(mock).CreateCluster(context.Background(), &ecscfg.ClusterConfig{Name: "any"}); err == nil {
		t.Fatal("expected error when DescribeClusters fails")
	}
}

// --- CreateService existence check ---

func TestCreateService_AlreadyExists(t *testing.T) {
	active := "ACTIVE"
	mock := &mockECS{
		describeServicesOut: &ecs.DescribeServicesOutput{
			Services: []types.Service{
				{ServiceName: aws.String("my-svc"), Status: &active},
			},
		},
	}
	err := testClient(mock).CreateService(context.Background(), &ecscfg.ServiceConfig{
		Name: "my-svc", Cluster: "c", TaskDefinition: "t:1", LaunchType: "FARGATE", DesiredCount: 1,
	})
	if err == nil {
		t.Fatal("expected error for existing service, got nil")
	}
	if mock.createServiceCalled {
		t.Error("SDK CreateService should not be called when service already exists")
	}
}

func TestCreateService_DoesNotExist(t *testing.T) {
	mock := &mockECS{
		describeServicesOut: &ecs.DescribeServicesOutput{Services: []types.Service{}},
	}
	err := testClient(mock).CreateService(context.Background(), &ecscfg.ServiceConfig{
		Name: "new-svc", Cluster: "c", TaskDefinition: "t:1", LaunchType: "FARGATE", DesiredCount: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mock.createServiceCalled {
		t.Error("SDK CreateService should be called for a new service")
	}
}

func TestCreateCluster_DefaultStrategiesAndServiceConnectNamespace(t *testing.T) {
	mock := &mockECS{describeClustersOut: &ecs.DescribeClustersOutput{}}
	cfg := &ecscfg.ClusterConfig{Name: "new-cluster", CapacityProviders: []string{"FARGATE"}, DefaultCapacityProviderStrategy: []ecscfg.CapacityProviderStrategyConfig{{CapacityProvider: "FARGATE", Weight: 1}}, ServiceConnectDefaultsNamespace: "prod.local"}
	if err := testClient(mock).CreateCluster(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if len(mock.createClusterInput.DefaultCapacityProviderStrategy) != 1 || aws.ToString(mock.createClusterInput.DefaultCapacityProviderStrategy[0].CapacityProvider) != "FARGATE" {
		t.Fatalf("default capacity provider strategy not passed: %#v", mock.createClusterInput.DefaultCapacityProviderStrategy)
	}
	if got := aws.ToString(mock.createClusterInput.ServiceConnectDefaults.Namespace); got != "prod.local" {
		t.Fatalf("Service Connect namespace = %q", got)
	}
}

func TestCreateServiceMapsAdvancedConfiguration(t *testing.T) {
	managed, execEnabled := true, true
	mock := &mockECS{describeServicesOut: &ecs.DescribeServicesOutput{}}
	cfg := &ecscfg.ServiceConfig{
		Name: "api", Cluster: "prod", TaskDefinition: "api:2", LaunchType: "FARGATE", DesiredCount: 2,
		CapacityProviderStrategy:      []ecscfg.CapacityProviderStrategyConfig{{CapacityProvider: "FARGATE", Weight: 1}},
		DeploymentConfiguration:       &ecscfg.DeploymentConfigurationConfig{DeploymentCircuitBreaker: &ecscfg.DeploymentCircuitBreakerConfig{Enable: true, Rollback: true}},
		LoadBalancers:                 []ecscfg.LoadBalancerConfig{{TargetGroupARN: "tg", ContainerName: "web", ContainerPort: 8080}},
		ServiceRegistries:             []ecscfg.ServiceRegistryConfig{{RegistryARN: "registry"}},
		HealthCheckGracePeriodSeconds: 30, EnableExecuteCommand: &execEnabled, EnableECSManagedTags: &managed,
		PropagateTags: "SERVICE", PlatformVersion: "LATEST", Tags: map[string]string{"env": "prod"},
		ServiceConnect: &ecscfg.ServiceConnectConfig{Enabled: true, Namespace: "prod.local", Services: []ecscfg.ServiceConnectServiceConfig{{PortName: "http", DiscoveryName: "api"}}},
	}
	if err := testClient(mock).CreateService(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	in := mock.createServiceInput
	if len(in.CapacityProviderStrategy) != 1 || in.LaunchType != "" {
		t.Fatalf("capacity strategy/launch type not mapped: %#v %q", in.CapacityProviderStrategy, in.LaunchType)
	}
	if len(in.LoadBalancers) != 1 || aws.ToString(in.LoadBalancers[0].TargetGroupArn) != "tg" {
		t.Fatalf("load balancer not mapped: %#v", in.LoadBalancers)
	}
	if in.DeploymentConfiguration == nil || in.DeploymentConfiguration.DeploymentCircuitBreaker == nil || !in.DeploymentConfiguration.DeploymentCircuitBreaker.Rollback {
		t.Fatalf("deployment safety config not mapped: %#v", in.DeploymentConfiguration)
	}
	if !in.EnableExecuteCommand || !in.EnableECSManagedTags || in.HealthCheckGracePeriodSeconds == nil || *in.HealthCheckGracePeriodSeconds != 30 {
		t.Fatalf("service options not mapped: %#v", in)
	}
	if len(in.Tags) != 1 || aws.ToString(in.Tags[0].Key) != "env" {
		t.Fatalf("tags not passed: %#v", in.Tags)
	}
	if in.ServiceConnectConfiguration == nil || !in.ServiceConnectConfiguration.Enabled {
		t.Fatalf("Service Connect not mapped: %#v", in.ServiceConnectConfiguration)
	}
}

func TestReconcileServiceAvoidsRevisionARNFalseDrift(t *testing.T) {
	active := "ACTIVE"
	mock := &mockECS{describeServicesOut: &ecs.DescribeServicesOutput{Services: []types.Service{{ServiceName: aws.String("api"), Status: &active, TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api:4"), DesiredCount: 1}}}}
	changed, err := testClient(mock).ReconcileService(context.Background(), &ecscfg.ServiceConfig{Name: "api", Cluster: "prod", TaskDefinition: "api:4", DesiredCount: 1, LaunchType: "FARGATE"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("family:revision and full ARN should resolve to the same task definition")
	}
	if mock.updateServiceInput != nil {
		t.Fatal("no update should be sent when the service matches")
	}
}

func TestDeployServicePreservesTaskDefinitionFields(t *testing.T) {
	faultInjection := true
	role := "arn:aws:iam::123456789012:role/task"
	mock := &mockECS{
		describeServicesOut: &ecs.DescribeServicesOutput{Services: []types.Service{{TaskDefinition: aws.String("api:4")}}},
		describeTaskDefOut: &ecs.DescribeTaskDefinitionOutput{Tags: []types.Tag{{Key: aws.String("owner"), Value: aws.String("platform")}}, TaskDefinition: &types.TaskDefinition{
			Family: aws.String("api"), ContainerDefinitions: []types.ContainerDefinition{{Name: aws.String("web"), Image: aws.String("old")}}, Cpu: aws.String("512"), Memory: aws.String("1024"), NetworkMode: types.NetworkModeAwsvpc, RequiresCompatibilities: []types.Compatibility{types.CompatibilityFargate},
			ExecutionRoleArn: &role, TaskRoleArn: &role, EnableFaultInjection: &faultInjection, IpcMode: types.IpcModeTask, PidMode: types.PidModeTask,
			RuntimePlatform: &types.RuntimePlatform{CpuArchitecture: types.CPUArchitectureArm64, OperatingSystemFamily: types.OSFamilyLinux},
		}},
		registerTaskDefOut: &ecs.RegisterTaskDefinitionOutput{TaskDefinition: &types.TaskDefinition{TaskDefinitionArn: aws.String("api:5")}},
	}
	if _, err := testClient(mock).DeployService(context.Background(), "prod", "api", "web", "new-image"); err != nil {
		t.Fatal(err)
	}
	in := mock.registerTaskDefIn
	if in == nil || in.EnableFaultInjection == nil || !*in.EnableFaultInjection {
		t.Fatalf("fault injection was dropped: %#v", in)
	}
	if in.RuntimePlatform == nil || in.RuntimePlatform.CpuArchitecture != types.CPUArchitectureArm64 {
		t.Fatalf("runtime platform was dropped: %#v", in.RuntimePlatform)
	}
	if in.IpcMode != types.IpcModeTask || in.PidMode != types.PidModeTask {
		t.Fatalf("namespace modes were dropped: %s %s", in.IpcMode, in.PidMode)
	}
	if len(in.Tags) != 1 || aws.ToString(in.Tags[0].Key) != "owner" {
		t.Fatalf("task definition tags were dropped: %#v", in.Tags)
	}
	if aws.ToString(in.ContainerDefinitions[0].Image) != "new-image" {
		t.Fatalf("container image not updated: %s", aws.ToString(in.ContainerDefinitions[0].Image))
	}
}
