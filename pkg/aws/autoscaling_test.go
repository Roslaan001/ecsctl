package aws

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

type mockApplicationAutoScaling struct {
	registerInput       *applicationautoscaling.RegisterScalableTargetInput
	registerErr         error
	policyInput         *applicationautoscaling.PutScalingPolicyInput
	policyErr           error
	targetsOut          *applicationautoscaling.DescribeScalableTargetsOutput
	policiesOut         *applicationautoscaling.DescribeScalingPoliciesOutput
	describeTargetCalls int
	describePolicyCalls int
}

func (m *mockApplicationAutoScaling) RegisterScalableTarget(_ context.Context, input *applicationautoscaling.RegisterScalableTargetInput, _ ...func(*applicationautoscaling.Options)) (*applicationautoscaling.RegisterScalableTargetOutput, error) {
	m.registerInput = input
	return &applicationautoscaling.RegisterScalableTargetOutput{}, m.registerErr
}

func (m *mockApplicationAutoScaling) PutScalingPolicy(_ context.Context, input *applicationautoscaling.PutScalingPolicyInput, _ ...func(*applicationautoscaling.Options)) (*applicationautoscaling.PutScalingPolicyOutput, error) {
	m.policyInput = input
	return &applicationautoscaling.PutScalingPolicyOutput{}, m.policyErr
}

func (m *mockApplicationAutoScaling) DescribeScalableTargets(_ context.Context, _ *applicationautoscaling.DescribeScalableTargetsInput, _ ...func(*applicationautoscaling.Options)) (*applicationautoscaling.DescribeScalableTargetsOutput, error) {
	m.describeTargetCalls++
	if m.targetsOut == nil {
		return &applicationautoscaling.DescribeScalableTargetsOutput{}, nil
	}
	return m.targetsOut, nil
}

func (m *mockApplicationAutoScaling) DescribeScalingPolicies(_ context.Context, _ *applicationautoscaling.DescribeScalingPoliciesInput, _ ...func(*applicationautoscaling.Options)) (*applicationautoscaling.DescribeScalingPoliciesOutput, error) {
	m.describePolicyCalls++
	if m.policiesOut == nil {
		return &applicationautoscaling.DescribeScalingPoliciesOutput{}, nil
	}
	return m.policiesOut, nil
}

func TestConfigureServiceAutoScalingDefaultsToCPU(t *testing.T) {
	mock := &mockApplicationAutoScaling{}
	client := &Client{autoscaling: mock}
	cfg := &ecscfg.ServiceConfig{
		Name: "api", Cluster: "cluster-a",
		AutoScaling: &ecscfg.ServiceAutoScalingConfig{MinCapacity: 1, MaxCapacity: 5},
	}
	if err := client.configureServiceAutoScaling(context.Background(), cfg); err != nil {
		t.Fatalf("configureServiceAutoScaling() error = %v", err)
	}
	if awssdk.ToString(mock.registerInput.ResourceId) != "service/cluster-a/api" || mock.registerInput.ServiceNamespace != types.ServiceNamespaceEcs || awssdk.ToInt32(mock.registerInput.MinCapacity) != 1 || awssdk.ToInt32(mock.registerInput.MaxCapacity) != 5 {
		t.Fatalf("RegisterScalableTarget input = %#v", mock.registerInput)
	}
	tracking := mock.policyInput.TargetTrackingScalingPolicyConfiguration
	if tracking == nil || awssdk.ToFloat64(tracking.TargetValue) != 60 || tracking.PredefinedMetricSpecification.PredefinedMetricType != types.MetricTypeECSServiceAverageCPUUtilization {
		t.Fatalf("PutScalingPolicy input = %#v", mock.policyInput)
	}
	if awssdk.ToString(mock.policyInput.PolicyName) != "ecsctl-api-target-tracking" || mock.policyInput.PolicyType != types.PolicyTypeTargetTrackingScaling {
		t.Fatalf("policy identity = %#v", mock.policyInput)
	}
}

func TestConfigureServiceAutoScalingMemoryMetric(t *testing.T) {
	mock := &mockApplicationAutoScaling{}
	client := &Client{autoscaling: mock}
	cfg := &ecscfg.ServiceConfig{Name: "api", Cluster: "cluster-a", AutoScaling: &ecscfg.ServiceAutoScalingConfig{Metric: "memory_utilization", TargetValue: 72.5}}
	if err := client.configureServiceAutoScaling(context.Background(), cfg); err != nil {
		t.Fatalf("configureServiceAutoScaling() error = %v", err)
	}
	tracking := mock.policyInput.TargetTrackingScalingPolicyConfiguration
	if awssdk.ToFloat64(tracking.TargetValue) != 72.5 || tracking.PredefinedMetricSpecification.PredefinedMetricType != types.MetricTypeECSServiceAverageMemoryUtilization {
		t.Fatalf("tracking policy = %#v", tracking)
	}
}

func TestConfigureServiceAutoScalingValidatesMetricBeforeMutation(t *testing.T) {
	mock := &mockApplicationAutoScaling{}
	client := &Client{autoscaling: mock}
	cfg := &ecscfg.ServiceConfig{Name: "api", Cluster: "cluster-a", AutoScaling: &ecscfg.ServiceAutoScalingConfig{Metric: "requests"}}
	if err := client.configureServiceAutoScaling(context.Background(), cfg); err == nil {
		t.Fatal("expected unsupported metric error")
	}
	if mock.registerInput != nil || mock.policyInput != nil {
		t.Fatal("unsupported metric made a scaling API call")
	}
}

func TestConfigureServiceAutoScalingPropagatesFailures(t *testing.T) {
	registerErr := errors.New("register failed")
	mock := &mockApplicationAutoScaling{registerErr: registerErr}
	client := &Client{autoscaling: mock}
	cfg := &ecscfg.ServiceConfig{Name: "api", Cluster: "cluster-a", AutoScaling: &ecscfg.ServiceAutoScalingConfig{}}
	if err := client.configureServiceAutoScaling(context.Background(), cfg); !errors.Is(err, registerErr) {
		t.Fatalf("register error = %v", err)
	}
	if mock.policyInput != nil {
		t.Fatal("PutScalingPolicy called after scalable-target registration failed")
	}

	policyErr := errors.New("policy failed")
	mock = &mockApplicationAutoScaling{policyErr: policyErr}
	client.autoscaling = mock
	if err := client.configureServiceAutoScaling(context.Background(), cfg); !errors.Is(err, policyErr) {
		t.Fatalf("policy error = %v", err)
	}
}

func TestServiceAutoScalingDriftIsFalseWhenTargetAndPolicyMatch(t *testing.T) {
	mock := &mockApplicationAutoScaling{
		targetsOut: &applicationautoscaling.DescribeScalableTargetsOutput{ScalableTargets: []types.ScalableTarget{{MinCapacity: awssdk.Int32(1), MaxCapacity: awssdk.Int32(5)}}},
		policiesOut: &applicationautoscaling.DescribeScalingPoliciesOutput{ScalingPolicies: []types.ScalingPolicy{{TargetTrackingScalingPolicyConfiguration: &types.TargetTrackingScalingPolicyConfiguration{
			TargetValue:                   awssdk.Float64(60),
			PredefinedMetricSpecification: &types.PredefinedMetricSpecification{PredefinedMetricType: types.MetricTypeECSServiceAverageCPUUtilization},
		}}}},
	}
	client := &Client{autoscaling: mock}
	cfg := &ecscfg.ServiceConfig{Name: "api", Cluster: "cluster-a", AutoScaling: &ecscfg.ServiceAutoScalingConfig{MinCapacity: 1, MaxCapacity: 5}}
	drift, err := client.serviceAutoScalingDrift(context.Background(), cfg)
	if err != nil || drift {
		t.Fatalf("serviceAutoScalingDrift() = (%v, %v), want (false, nil)", drift, err)
	}
}
