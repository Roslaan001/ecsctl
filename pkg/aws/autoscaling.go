package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

func (c *Client) serviceAutoScalingDrift(ctx context.Context, cfg *ecscfg.ServiceConfig) (bool, error) {
	if c.autoscaling == nil {
		return false, fmt.Errorf("application auto scaling client is unavailable")
	}
	resourceID := fmt.Sprintf("service/%s/%s", cfg.Cluster, cfg.Name)
	targets, err := c.autoscaling.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace:  types.ServiceNamespaceEcs,
		ScalableDimension: types.ScalableDimensionECSServiceDesiredCount,
		ResourceIds:       []string{resourceID},
	})
	if err != nil {
		return false, fmt.Errorf("describing scalable target: %w", err)
	}
	wantMin, wantMax := cfg.AutoScaling.MinCapacity, cfg.AutoScaling.MaxCapacity
	if len(targets.ScalableTargets) == 0 || aws.ToInt32(targets.ScalableTargets[0].MinCapacity) != wantMin || aws.ToInt32(targets.ScalableTargets[0].MaxCapacity) != wantMax {
		return true, nil
	}
	metric, err := scalingMetric(cfg.AutoScaling.Metric)
	if err != nil {
		return false, err
	}
	targetValue := cfg.AutoScaling.TargetValue
	if targetValue == 0 {
		targetValue = 60
	}
	policies, err := c.autoscaling.DescribeScalingPolicies(ctx, &applicationautoscaling.DescribeScalingPoliciesInput{
		ServiceNamespace:  types.ServiceNamespaceEcs,
		ScalableDimension: types.ScalableDimensionECSServiceDesiredCount,
		ResourceId:        aws.String(resourceID),
		PolicyNames:       []string{"ecsctl-" + cfg.Name + "-target-tracking"},
	})
	if err != nil {
		return false, fmt.Errorf("describing scaling policy: %w", err)
	}
	for _, policy := range policies.ScalingPolicies {
		configuration := policy.TargetTrackingScalingPolicyConfiguration
		if configuration != nil && aws.ToFloat64(configuration.TargetValue) == targetValue && configuration.PredefinedMetricSpecification != nil && configuration.PredefinedMetricSpecification.PredefinedMetricType == metric {
			return false, nil
		}
	}
	return true, nil
}

func (c *Client) configureServiceAutoScaling(ctx context.Context, cfg *ecscfg.ServiceConfig) error {
	if c.autoscaling == nil {
		return fmt.Errorf("application auto scaling client is unavailable")
	}
	metricType, err := scalingMetric(cfg.AutoScaling.Metric)
	if err != nil {
		return err
	}
	resourceID := fmt.Sprintf("service/%s/%s", cfg.Cluster, cfg.Name)
	_, err = c.autoscaling.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
		ServiceNamespace:  types.ServiceNamespaceEcs,
		ScalableDimension: types.ScalableDimensionECSServiceDesiredCount,
		ResourceId:        aws.String(resourceID),
		MinCapacity:       aws.Int32(cfg.AutoScaling.MinCapacity),
		MaxCapacity:       aws.Int32(cfg.AutoScaling.MaxCapacity),
	})
	if err != nil {
		return fmt.Errorf("registering scalable target: %w", err)
	}
	target := cfg.AutoScaling.TargetValue
	if target == 0 {
		target = 60
	}
	_, err = c.autoscaling.PutScalingPolicy(ctx, &applicationautoscaling.PutScalingPolicyInput{
		PolicyName:        aws.String("ecsctl-" + cfg.Name + "-target-tracking"),
		PolicyType:        types.PolicyTypeTargetTrackingScaling,
		ServiceNamespace:  types.ServiceNamespaceEcs,
		ScalableDimension: types.ScalableDimensionECSServiceDesiredCount,
		ResourceId:        aws.String(resourceID),
		TargetTrackingScalingPolicyConfiguration: &types.TargetTrackingScalingPolicyConfiguration{
			TargetValue:                   aws.Float64(target),
			PredefinedMetricSpecification: &types.PredefinedMetricSpecification{PredefinedMetricType: metricType},
		},
	})
	if err != nil {
		return fmt.Errorf("putting target tracking policy: %w", err)
	}
	return nil
}

func scalingMetric(value string) (types.MetricType, error) {
	metric := strings.ToLower(strings.ReplaceAll(value, "_", ""))
	switch metric {
	case "", "cpu", "cpuutilization", "ecsserviceaveragecpuutilization":
		return types.MetricTypeECSServiceAverageCPUUtilization, nil
	case "memory", "memoryutilization", "ecsserviceaveragememoryutilization":
		return types.MetricTypeECSServiceAverageMemoryUtilization, nil
	default:
		return "", fmt.Errorf("unsupported autoScaling.metric %q; use CPU or Memory", value)
	}
}
