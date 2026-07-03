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
	describeServicesOut *ecs.DescribeServicesOutput
	describeServicesErr error
	createServiceCalled bool
	createServiceErr    error
}

func (m *mockECS) DescribeClusters(_ context.Context, _ *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	return m.describeClustersOut, m.describeClustersErr
}
func (m *mockECS) CreateCluster(_ context.Context, _ *ecs.CreateClusterInput, _ ...func(*ecs.Options)) (*ecs.CreateClusterOutput, error) {
	m.createClusterCalled = true
	return &ecs.CreateClusterOutput{}, m.createClusterErr
}
func (m *mockECS) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	return m.describeServicesOut, m.describeServicesErr
}
func (m *mockECS) CreateService(_ context.Context, _ *ecs.CreateServiceInput, _ ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error) {
	m.createServiceCalled = true
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
func (m *mockECS) UpdateService(_ context.Context, _ *ecs.UpdateServiceInput, _ ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error) {
	panic("unexpected: UpdateService")
}
func (m *mockECS) DescribeTaskDefinition(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	panic("unexpected: DescribeTaskDefinition")
}
func (m *mockECS) RegisterTaskDefinition(_ context.Context, _ *ecs.RegisterTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error) {
	panic("unexpected: RegisterTaskDefinition")
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
