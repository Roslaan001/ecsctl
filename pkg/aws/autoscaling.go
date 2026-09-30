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

func (c *Client) configureServiceAutoScaling(ctx context.Context, cfg *ecscfg.ServiceConfig) error {
	if c.autoscaling == nil {
		return fmt.Errorf("Application Auto Scaling client is unavailable")
	}
	resourceID := fmt.Sprintf("service/%s/%s", cfg.Cluster, cfg.Name)
	_, err := c.autoscaling.RegisterScalableTarget(ctx, &applicationautoscaling.RegisterScalableTargetInput{
		ServiceNamespace:  types.ServiceNamespaceEcs,
		ScalableDimension: types.ScalableDimensionECSServiceDesiredCount,
		ResourceId:        aws.String(resourceID),
		MinCapacity:       aws.Int32(cfg.AutoScaling.MinCapacity),
		MaxCapacity:       aws.Int32(cfg.AutoScaling.MaxCapacity),
	})
	if err != nil {
		return fmt.Errorf("registering scalable target: %w", err)
	}
	metric := strings.ToLower(strings.ReplaceAll(cfg.AutoScaling.Metric, "_", ""))
	metricType := types.MetricTypeECSServiceAverageCPUUtilization
	switch metric {
	case "", "cpu", "cpuutilization", "ecsserviceaveragecpuutilization":
	case "memory", "memoryutilization", "ecsserviceaveragememoryutilization":
		metricType = types.MetricTypeECSServiceAverageMemoryUtilization
	default:
		return fmt.Errorf("unsupported autoScaling.metric %q; use CPU or Memory", cfg.AutoScaling.Metric)
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
