package aws

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

func TestCreateGateway(t *testing.T) {
	mock := &mockECS{
		createGatewayOut: &ecs.CreateGatewayOutput{
			GatewayArn: aws.String("arn:aws:ecs:us-east-1:123456789012:gateway/test-gw"),
			Name:       aws.String("test-gw"),
			Status:     types.GatewayStatusActive,
		},
	}
	client := testClient(mock)

	delProtection := true
	cfg := &ecscfg.GatewayConfig{
		Name:                  "test-gw",
		InfrastructureRoleARN: "arn:aws:iam::123456789012:role/ecsInfraRole",
		DeletionProtection:    &delProtection,
		Clusters:              []string{"prod-cluster"},
		Configuration: &ecscfg.GatewayConfigurationConfig{
			HostSharedAlb: &ecscfg.HostSharedAlbConfig{
				Scheme:      "internet-facing",
				MaxAlbCount: 5,
				Network: &ecscfg.GatewayNetworkConfig{
					Subnets:              []string{"subnet-1", "subnet-2"},
					InboundSecurityGroup: "sg-12345",
					IPAddressType:        "dualstack",
				},
			},
		},
		Tags: map[string]string{"Env": "Test"},
	}

	arn, err := client.CreateGateway(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error creating gateway: %v", err)
	}
	if arn != "arn:aws:ecs:us-east-1:123456789012:gateway/test-gw" {
		t.Errorf("unexpected gateway ARN: %q", arn)
	}
	if mock.createGatewayIn == nil {
		t.Fatalf("CreateGateway was not called on ECS client")
	}
	if aws.ToString(mock.createGatewayIn.Name) != "test-gw" {
		t.Errorf("expected name 'test-gw', got %q", aws.ToString(mock.createGatewayIn.Name))
	}
	if mock.associateGatewayIn == nil {
		t.Fatalf("expected AssociateGatewayWithCluster to be called for prod-cluster")
	}
	if aws.ToString(mock.associateGatewayIn.Cluster) != "prod-cluster" {
		t.Errorf("expected cluster 'prod-cluster', got %q", aws.ToString(mock.associateGatewayIn.Cluster))
	}
}

func TestCreateGateway_AutoProvisionRole(t *testing.T) {
	mock := &mockECS{
		createGatewayOut: &ecs.CreateGatewayOutput{
			GatewayArn: aws.String("arn:aws:ecs:us-east-1:123456789012:gateway/auto-role-gw"),
			Name:       aws.String("auto-role-gw"),
			Status:     types.GatewayStatusActive,
		},
	}
	client := testClient(mock)

	cfg := &ecscfg.GatewayConfig{
		Name: "auto-role-gw",
	}

	arn, err := client.CreateGateway(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error creating gateway with auto role: %v", err)
	}
	if arn != "arn:aws:ecs:us-east-1:123456789012:gateway/auto-role-gw" {
		t.Errorf("unexpected gateway ARN: %q", arn)
	}
	if mock.createGatewayIn == nil {
		t.Fatalf("CreateGateway was not called on ECS client")
	}
	if aws.ToString(mock.createGatewayIn.InfrastructureRoleArn) != "arn:aws:iam::123456789012:role/ecsInfrastructureRoleForGateways" {
		t.Errorf("expected default role ARN, got %q", aws.ToString(mock.createGatewayIn.InfrastructureRoleArn))
	}
}

func TestDescribeGateway(t *testing.T) {
	mock := &mockECS{
		describeGatewayOut: &ecs.DescribeGatewayOutput{
			Gateway: &types.Gateway{
				GatewayArn: aws.String("arn:aws:ecs:us-east-1:123456789012:gateway/my-gw"),
				Name:       aws.String("my-gw"),
				Status:     types.GatewayStatusActive,
			},
		},
	}
	client := testClient(mock)

	gw, err := client.DescribeGateway(context.Background(), "my-gw")
	if err != nil {
		t.Fatalf("DescribeGateway failed: %v", err)
	}
	if aws.ToString(gw.Name) != "my-gw" {
		t.Errorf("expected name 'my-gw', got %q", aws.ToString(gw.Name))
	}
}

func TestDeleteGateway(t *testing.T) {
	mock := &mockECS{}
	client := testClient(mock)

	err := client.DeleteGateway(context.Background(), "my-gw")
	if err != nil {
		t.Fatalf("DeleteGateway failed: %v", err)
	}
	if mock.deleteGatewayIn == nil || aws.ToString(mock.deleteGatewayIn.Gateway) != "my-gw" {
		t.Errorf("DeleteGateway was not called with 'my-gw'")
	}
}

func TestListGateways(t *testing.T) {
	now := time.Now()
	mock := &mockECS{
		listGatewaysOut: &ecs.ListGatewaysOutput{
			Gateways: []types.GatewaySummary{
				{
					Name:       aws.String("gw-1"),
					GatewayArn: aws.String("arn:aws:ecs:us-east-1:123456789012:gateway/gw-1"),
					Status:     types.GatewayStatusActive,
					CreatedAt:  &now,
				},
			},
		},
	}
	client := testClient(mock)

	gws, err := client.ListGateways(context.Background())
	if err != nil {
		t.Fatalf("ListGateways failed: %v", err)
	}
	if len(gws) != 1 || aws.ToString(gws[0].Name) != "gw-1" {
		t.Errorf("unexpected list gateways output: %+v", gws)
	}
}

func TestAssociateAndDisassociateGateway(t *testing.T) {
	mock := &mockECS{}
	client := testClient(mock)

	if err := client.AssociateGatewayWithCluster(context.Background(), "gw-1", "cluster-1"); err != nil {
		t.Fatalf("AssociateGatewayWithCluster failed: %v", err)
	}
	if mock.associateGatewayIn == nil || aws.ToString(mock.associateGatewayIn.Cluster) != "cluster-1" {
		t.Errorf("unexpected association input: %+v", mock.associateGatewayIn)
	}

	if err := client.DisassociateGatewayFromCluster(context.Background(), "gw-1", "cluster-1"); err != nil {
		t.Fatalf("DisassociateGatewayFromCluster failed: %v", err)
	}
	if mock.disassociateGatewayIn == nil || aws.ToString(mock.disassociateGatewayIn.Cluster) != "cluster-1" {
		t.Errorf("unexpected disassociation input: %+v", mock.disassociateGatewayIn)
	}
}

func TestCreateService_WithGatewayConnector(t *testing.T) {
	mock := &mockECS{
		describeServicesOut: &ecs.DescribeServicesOutput{}, // empty -> doesn't exist yet
	}
	client := testClient(mock)

	cfg := &ecscfg.ServiceConfig{
		Name:           "api-svc",
		Cluster:        "my-cluster",
		TaskDefinition: "api:1",
		GatewayConnector: &ecscfg.GatewayConnectorConfig{
			PortName: "http",
			HealthCheck: &ecscfg.GatewayConnectorHealthCheckConfig{
				Path:            "/healthz",
				Protocol:        "HTTP",
				IntervalSeconds: 15,
			},
		},
	}

	if err := client.CreateService(context.Background(), cfg); err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}
	if mock.createServiceInput == nil {
		t.Fatalf("CreateService was not called on ECS client")
	}
	if len(mock.createServiceInput.GatewayConnectors) != 1 {
		t.Fatalf("expected 1 gateway connector, got %d", len(mock.createServiceInput.GatewayConnectors))
	}
	gc := mock.createServiceInput.GatewayConnectors[0]
	if aws.ToString(gc.PortName) != "http" {
		t.Errorf("expected portName 'http', got %q", aws.ToString(gc.PortName))
	}
	if gc.HealthCheck == nil || aws.ToString(gc.HealthCheck.HealthCheckPath) != "/healthz" {
		t.Errorf("unexpected health check on gateway connector: %+v", gc.HealthCheck)
	}
}
