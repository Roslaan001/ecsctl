package aws

import (
	"context"
	"errors"
	"reflect"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

func TestCreateExpressServiceMapsImageConfiguration(t *testing.T) {
	mock := &mockECS{
		createExpressOut: &ecs.CreateExpressGatewayServiceOutput{
			Service: &types.ECSExpressGatewayService{ServiceArn: awssdk.String("arn:aws:ecs:us-east-1:123456789012:express-gateway-service/demo")},
		},
	}
	cfg := &ecscfg.ExpressServiceConfig{
		ServiceName:           "demo",
		Cluster:               "cluster-a",
		InfrastructureRoleARN: "arn:aws:iam::123456789012:role/infra",
		ExecutionRoleARN:      "arn:aws:iam::123456789012:role/execution",
		TaskRoleARN:           "arn:aws:iam::123456789012:role/task",
		Image:                 "example/app:latest",
		ContainerPort:         8080,
		Command:               []string{"./serve"},
		Environment:           map[string]string{"MODE": "test"},
		Secrets:               map[string]string{"TOKEN": "arn:aws:ssm:us-east-1:123456789012:parameter/token"},
		AWSLogsConfiguration:  &ecscfg.ExpressAWSLogsConfig{LogGroup: "/ecs/demo", LogStreamPrefix: "app"},
		RepositoryCredentials: &ecscfg.ExpressRepositoryCredentialsConfig{CredentialsParameter: "arn:aws:secretsmanager:us-east-1:123456789012:secret:registry"},
		CPU:                   "512",
		Memory:                "1024",
		CPUArchitecture:       "ARM64",
		HealthCheckPath:       "/health",
		Subnets:               []string{"subnet-a", "subnet-b"},
		SecurityGroups:        []string{"sg-a"},
		MinTaskCount:          1,
		MaxTaskCount:          4,
		ScalingMetric:         string(types.ExpressGatewayServiceScalingMetricAverageCPUUtilization),
		ScalingTargetValue:    65,
		Tags:                  map[string]string{"env": "test"},
	}

	arn, err := testClient(mock).CreateExpressService(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CreateExpressService() error = %v", err)
	}
	if arn != "arn:aws:ecs:us-east-1:123456789012:express-gateway-service/demo" {
		t.Fatalf("service ARN = %q", arn)
	}
	in := mock.createExpressInput
	if awssdk.ToString(in.ServiceName) != cfg.ServiceName || awssdk.ToString(in.Cluster) != cfg.Cluster {
		t.Fatalf("service identity not mapped: %#v", in)
	}
	if awssdk.ToString(in.InfrastructureRoleArn) != cfg.InfrastructureRoleARN || awssdk.ToString(in.ExecutionRoleArn) != cfg.ExecutionRoleARN || awssdk.ToString(in.TaskRoleArn) != cfg.TaskRoleARN {
		t.Fatalf("role ARNs not mapped: %#v", in)
	}
	if awssdk.ToString(in.Cpu) != cfg.CPU || awssdk.ToString(in.Memory) != cfg.Memory || in.CpuArchitecture != types.ExpressCpuArchitectureArm64 {
		t.Fatalf("CPU, memory, or architecture not mapped: %#v", in)
	}
	if in.PrimaryContainer == nil || awssdk.ToString(in.PrimaryContainer.Image) != cfg.Image || awssdk.ToInt32(in.PrimaryContainer.ContainerPort) != cfg.ContainerPort || !reflect.DeepEqual(in.PrimaryContainer.Command, cfg.Command) {
		t.Fatalf("primary container not mapped: %#v", in.PrimaryContainer)
	}
	if len(in.PrimaryContainer.Environment) != 1 || awssdk.ToString(in.PrimaryContainer.Environment[0].Name) != "MODE" || awssdk.ToString(in.PrimaryContainer.Environment[0].Value) != "test" {
		t.Fatalf("environment not mapped: %#v", in.PrimaryContainer.Environment)
	}
	if len(in.PrimaryContainer.Secrets) != 1 || awssdk.ToString(in.PrimaryContainer.Secrets[0].Name) != "TOKEN" {
		t.Fatalf("secrets not mapped: %#v", in.PrimaryContainer.Secrets)
	}
	if in.PrimaryContainer.AwsLogsConfiguration == nil || awssdk.ToString(in.PrimaryContainer.AwsLogsConfiguration.LogGroup) != "/ecs/demo" || awssdk.ToString(in.PrimaryContainer.AwsLogsConfiguration.LogStreamPrefix) != "app" {
		t.Fatalf("CloudWatch log configuration not mapped: %#v", in.PrimaryContainer.AwsLogsConfiguration)
	}
	if in.PrimaryContainer.RepositoryCredentials == nil || awssdk.ToString(in.PrimaryContainer.RepositoryCredentials.CredentialsParameter) != cfg.RepositoryCredentials.CredentialsParameter {
		t.Fatalf("private registry credentials not mapped: %#v", in.PrimaryContainer.RepositoryCredentials)
	}
	if in.ScalingTarget == nil || awssdk.ToInt32(in.ScalingTarget.MinTaskCount) != 1 || awssdk.ToInt32(in.ScalingTarget.MaxTaskCount) != 4 || awssdk.ToInt32(in.ScalingTarget.AutoScalingTargetValue) != 65 {
		t.Fatalf("scaling target not mapped: %#v", in.ScalingTarget)
	}
	if in.NetworkConfiguration == nil || !reflect.DeepEqual(in.NetworkConfiguration.Subnets, cfg.Subnets) || !reflect.DeepEqual(in.NetworkConfiguration.SecurityGroups, cfg.SecurityGroups) {
		t.Fatalf("network configuration not mapped: %#v", in.NetworkConfiguration)
	}
	if awssdk.ToString(in.HealthCheckPath) != cfg.HealthCheckPath || len(in.Tags) != 1 || in.Tags[0].Key == nil || *in.Tags[0].Key != "env" {
		t.Fatalf("health check or tags not mapped: %#v", in)
	}
}

func TestCreateExpressServiceTaskDefinitionMode(t *testing.T) {
	mock := &mockECS{createExpressOut: &ecs.CreateExpressGatewayServiceOutput{Service: &types.ECSExpressGatewayService{ServiceArn: awssdk.String("service-arn")}}}
	cfg := &ecscfg.ExpressServiceConfig{ServiceName: "demo", TaskDefinitionARN: "task-definition-arn", InfrastructureRoleARN: "infra-role"}

	if _, err := testClient(mock).CreateExpressService(context.Background(), cfg); err != nil {
		t.Fatalf("CreateExpressService() error = %v", err)
	}
	in := mock.createExpressInput
	if awssdk.ToString(in.TaskDefinitionArn) != cfg.TaskDefinitionARN {
		t.Fatalf("task definition ARN = %q", awssdk.ToString(in.TaskDefinitionArn))
	}
	if in.PrimaryContainer != nil || in.ExecutionRoleArn != nil || in.TaskRoleArn != nil || in.Cpu != nil || in.Memory != nil || in.CpuArchitecture != "" {
		t.Fatalf("image-mode members should be omitted when a task definition is supplied: %#v", in)
	}
}

func TestCreateExpressServiceErrors(t *testing.T) {
	t.Run("API error", func(t *testing.T) {
		wantErr := errors.New("create failed")
		mock := &mockECS{createExpressErr: wantErr}
		_, err := testClient(mock).CreateExpressService(context.Background(), &ecscfg.ExpressServiceConfig{ServiceName: "demo"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want wrapped %v", err, wantErr)
		}
	})
	t.Run("empty response", func(t *testing.T) {
		_, err := testClient(&mockECS{}).CreateExpressService(context.Background(), &ecscfg.ExpressServiceConfig{ServiceName: "demo"})
		if err == nil {
			t.Fatal("expected error for empty service response")
		}
	})
}

func TestUpdateExpressServiceMapsConfiguredFields(t *testing.T) {
	mock := &mockECS{}
	cfg := &ecscfg.ExpressServiceConfig{Image: "example/updated:latest", ContainerPort: 9090, HealthCheckPath: "/ready", CPU: "1024", Memory: "2048", CPUArchitecture: "X86_64", MinTaskCount: 2, MaxTaskCount: 6}

	if err := testClient(mock).UpdateExpressService(context.Background(), "service-arn", cfg); err != nil {
		t.Fatalf("UpdateExpressService() error = %v", err)
	}
	in := mock.updateExpressInput
	if awssdk.ToString(in.ServiceArn) != "service-arn" || in.PrimaryContainer == nil || awssdk.ToString(in.PrimaryContainer.Image) != cfg.Image {
		t.Fatalf("service or container not mapped: %#v", in)
	}
	if awssdk.ToString(in.HealthCheckPath) != cfg.HealthCheckPath || awssdk.ToString(in.Cpu) != cfg.CPU || awssdk.ToString(in.Memory) != cfg.Memory || in.CpuArchitecture != types.ExpressCpuArchitectureX8664 {
		t.Fatalf("update fields not mapped: %#v", in)
	}
	if in.ScalingTarget == nil || awssdk.ToInt32(in.ScalingTarget.MinTaskCount) != cfg.MinTaskCount || awssdk.ToInt32(in.ScalingTarget.MaxTaskCount) != cfg.MaxTaskCount {
		t.Fatalf("scaling target not mapped: %#v", in.ScalingTarget)
	}
}

func TestDescribeExpressServiceIncludesTags(t *testing.T) {
	want := &types.ECSExpressGatewayService{ServiceArn: awssdk.String("service-arn")}
	mock := &mockECS{describeExpressOut: &ecs.DescribeExpressGatewayServiceOutput{Service: want}}
	got, err := testClient(mock).DescribeExpressService(context.Background(), "service-arn")
	if err != nil {
		t.Fatalf("DescribeExpressService() error = %v", err)
	}
	if got != want {
		t.Fatalf("service = %#v, want %#v", got, want)
	}
	if awssdk.ToString(mock.describeExpressIn.ServiceArn) != "service-arn" || !reflect.DeepEqual(mock.describeExpressIn.Include, []types.ExpressGatewayServiceInclude{types.ExpressGatewayServiceIncludeTags}) {
		t.Fatalf("describe request not mapped: %#v", mock.describeExpressIn)
	}
}

func TestDeleteExpressService(t *testing.T) {
	wantErr := errors.New("delete failed")
	mock := &mockECS{deleteExpressErr: wantErr}
	err := testClient(mock).DeleteExpressService(context.Background(), "service-arn")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if awssdk.ToString(mock.deleteExpressInput.ServiceArn) != "service-arn" {
		t.Fatalf("delete request = %#v", mock.deleteExpressInput)
	}
}

func TestListExpressServicesFiltersECSManagedResources(t *testing.T) {
	mock := &mockECS{
		listServicesOut:    &ecs.ListServicesOutput{ServiceArns: []string{"express-arn"}},
		describeExpressOut: &ecs.DescribeExpressGatewayServiceOutput{Service: &types.ECSExpressGatewayService{ServiceArn: awssdk.String("express-arn"), ServiceName: awssdk.String("api"), Cluster: awssdk.String("prod")}},
	}
	services, err := testClient(mock).ListExpressServices(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 || awssdk.ToString(services[0].ServiceName) != "api" {
		t.Fatalf("services = %#v", services)
	}
	if awssdk.ToString(mock.listServicesIn.Cluster) != "prod" || mock.listServicesIn.ResourceManagementType != types.ResourceManagementTypeEcs {
		t.Fatalf("list filter = %#v", mock.listServicesIn)
	}
}
