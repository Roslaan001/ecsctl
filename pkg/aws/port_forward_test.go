package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type mockSSM struct {
	getParameterOut  *ssm.GetParameterOutput
	getParameterErr  error
	startSessionIn   *ssm.StartSessionInput
	startSessionOut  *ssm.StartSessionOutput
	startSessionErr  error
	startSessionDone bool
}

func (m *mockSSM) GetParameter(_ context.Context, _ *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return m.getParameterOut, m.getParameterErr
}

func (m *mockSSM) StartSession(_ context.Context, in *ssm.StartSessionInput, _ ...func(*ssm.Options)) (*ssm.StartSessionOutput, error) {
	m.startSessionDone = true
	m.startSessionIn = in
	if m.startSessionOut == nil {
		m.startSessionOut = &ssm.StartSessionOutput{
			SessionId:  awssdk.String("session-123"),
			StreamUrl:  awssdk.String("wss://ssmmessages.us-east-1.amazonaws.com/test"),
			TokenValue: awssdk.String("token-xyz"),
		}
	}
	return m.startSessionOut, m.startSessionErr
}

func TestPortForward_Validation(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name    string
		opts    PortForwardOptions
		wantErr string
	}{
		{
			name:    "missing cluster",
			opts:    PortForwardOptions{LocalPort: 8080, ContainerPort: 80},
			wantErr: "cluster is required",
		},
		{
			name:    "invalid local port zero",
			opts:    PortForwardOptions{Cluster: "c", LocalPort: 0, ContainerPort: 80},
			wantErr: "invalid local port",
		},
		{
			name:    "invalid local port too high",
			opts:    PortForwardOptions{Cluster: "c", LocalPort: 70000, ContainerPort: 80},
			wantErr: "invalid local port",
		},
		{
			name:    "invalid container port negative",
			opts:    PortForwardOptions{Cluster: "c", LocalPort: 8080, ContainerPort: -1},
			wantErr: "invalid container port",
		},
		{
			name:    "missing service and task",
			opts:    PortForwardOptions{Cluster: "c", LocalPort: 8080, ContainerPort: 80},
			wantErr: "must specify either a service name or a task ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.PortForward(context.Background(), tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestPortForward_NoRunningTasks(t *testing.T) {
	mockEcs := &mockECS{
		listTasksOut: &ecs.ListTasksOutput{
			TaskArns: []string{},
		},
	}
	client := &Client{ecs: mockEcs}

	err := client.PortForward(context.Background(), PortForwardOptions{
		Cluster:       "test-cluster",
		Service:       "test-service",
		LocalPort:     8080,
		ContainerPort: 80,
	})
	if err == nil || !strings.Contains(err.Error(), "no running tasks found") {
		t.Fatalf("expected 'no running tasks found', got %v", err)
	}
}

func TestPortForward_TaskNotFound(t *testing.T) {
	mockEcs := &mockECS{
		describeTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []types.Task{},
		},
	}
	client := &Client{ecs: mockEcs}

	err := client.PortForward(context.Background(), PortForwardOptions{
		Cluster:       "test-cluster",
		TaskID:        "task-123",
		LocalPort:     8080,
		ContainerPort: 80,
	})
	if err == nil || !strings.Contains(err.Error(), "not found in cluster") {
		t.Fatalf("expected 'not found in cluster', got %v", err)
	}
}

func TestPortForward_ContainerNotFound(t *testing.T) {
	mockEcs := &mockECS{
		describeTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []types.Task{
				{
					Containers: []types.Container{
						{Name: awssdk.String("web"), RuntimeId: awssdk.String("web-runtime")},
					},
				},
			},
		},
	}
	client := &Client{ecs: mockEcs}

	err := client.PortForward(context.Background(), PortForwardOptions{
		Cluster:       "test-cluster",
		TaskID:        "task-123",
		Container:     "db",
		LocalPort:     5432,
		ContainerPort: 5432,
	})
	if err == nil || !strings.Contains(err.Error(), `container "db" not found`) {
		t.Fatalf("expected 'container \"db\" not found', got %v", err)
	}
}

func TestPortForward_ContainerMissingRuntimeID(t *testing.T) {
	status := "PENDING"
	mockEcs := &mockECS{
		describeTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []types.Task{
				{
					Containers: []types.Container{
						{Name: awssdk.String("web"), LastStatus: &status},
					},
				},
			},
		},
	}
	client := &Client{ecs: mockEcs}

	err := client.PortForward(context.Background(), PortForwardOptions{
		Cluster:       "test-cluster",
		TaskID:        "task-123",
		LocalPort:     8080,
		ContainerPort: 80,
	})
	if err == nil || !strings.Contains(err.Error(), "has no runtime ID") {
		t.Fatalf("expected 'has no runtime ID', got %v", err)
	}
}

func TestPortForward_MissingPlugin(t *testing.T) {
	mockEcs := &mockECS{
		describeTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []types.Task{
				{
					Containers: []types.Container{
						{Name: awssdk.String("web"), RuntimeId: awssdk.String("web-runtime-id")},
					},
				},
			},
		},
	}
	client := &Client{ecs: mockEcs, ssm: &mockSSM{}}

	origLookPath := sessionLookPath
	sessionLookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}
	defer func() { sessionLookPath = origLookPath }()

	err := client.PortForward(context.Background(), PortForwardOptions{
		Cluster:       "test-cluster",
		TaskID:        "task-123",
		LocalPort:     8080,
		ContainerPort: 80,
	})
	if err == nil || !strings.Contains(err.Error(), "session-manager-plugin not found in PATH") {
		t.Fatalf("expected plugin not found error, got %v", err)
	}
}

func TestPortForward_Success_Service(t *testing.T) {
	taskArn := "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/12345678-1234-1234-1234-123456789abc"
	mockEcs := &mockECS{
		listTasksOut: &ecs.ListTasksOutput{
			TaskArns: []string{taskArn},
		},
		describeTasksOut: &ecs.DescribeTasksOutput{
			Tasks: []types.Task{
				{
					Containers: []types.Container{
						{Name: awssdk.String("web"), RuntimeId: awssdk.String("runtime-web-123")},
					},
				},
			},
		},
	}
	mockSsm := &mockSSM{}
	client := &Client{
		ecs:    mockEcs,
		ssm:    mockSsm,
		region: "us-east-1",
	}

	origLookPath := sessionLookPath
	origRunner := sessionPluginRunner
	sessionLookPath = func(file string) (string, error) {
		return "/usr/local/bin/session-manager-plugin", nil
	}

	var capturedPath string
	var capturedArgs []string
	sessionPluginRunner = func(ctx context.Context, pluginPath string, args []string) error {
		capturedPath = pluginPath
		capturedArgs = args
		return nil
	}
	defer func() {
		sessionLookPath = origLookPath
		sessionPluginRunner = origRunner
	}()

	err := client.PortForward(context.Background(), PortForwardOptions{
		Cluster:       "my-cluster",
		Service:       "my-service",
		LocalPort:     8080,
		ContainerPort: 80,
		Profile:       "my-profile",
	})
	if err != nil {
		t.Fatalf("PortForward() unexpected error: %v", err)
	}

	// Verify SSM StartSession call
	if !mockSsm.startSessionDone {
		t.Fatal("expected SSM StartSession to be called")
	}
	wantTarget := "ecs:my-cluster_12345678-1234-1234-1234-123456789abc_runtime-web-123"
	if awssdk.ToString(mockSsm.startSessionIn.Target) != wantTarget {
		t.Errorf("target: got %q, want %q", awssdk.ToString(mockSsm.startSessionIn.Target), wantTarget)
	}
	if awssdk.ToString(mockSsm.startSessionIn.DocumentName) != "AWS-StartPortForwardingSession" {
		t.Errorf("document: got %q, want AWS-StartPortForwardingSession", awssdk.ToString(mockSsm.startSessionIn.DocumentName))
	}
	if mockSsm.startSessionIn.Parameters["portNumber"][0] != "80" {
		t.Errorf("container port: got %s, want 80", mockSsm.startSessionIn.Parameters["portNumber"][0])
	}
	if mockSsm.startSessionIn.Parameters["localPortNumber"][0] != "8080" {
		t.Errorf("local port: got %s, want 8080", mockSsm.startSessionIn.Parameters["localPortNumber"][0])
	}

	// Verify plugin runner execution
	if capturedPath != "/usr/local/bin/session-manager-plugin" {
		t.Errorf("plugin path: got %q", capturedPath)
	}
	if len(capturedArgs) != 6 {
		t.Fatalf("expected 6 arguments, got %d: %v", len(capturedArgs), capturedArgs)
	}
	if capturedArgs[1] != "us-east-1" {
		t.Errorf("region: got %q, want us-east-1", capturedArgs[1])
	}
	if capturedArgs[2] != "StartSession" {
		t.Errorf("action: got %q, want StartSession", capturedArgs[2])
	}
	if capturedArgs[3] != "my-profile" {
		t.Errorf("profile: got %q, want my-profile", capturedArgs[3])
	}
	if capturedArgs[5] != "https://ssm.us-east-1.amazonaws.com" {
		t.Errorf("endpoint: got %q", capturedArgs[5])
	}
}
