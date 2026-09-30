package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
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

func (c *Client) DescribeExpressService(ctx context.Context, arn string) (*types.ECSExpressGatewayService, error) {
	out, err := c.ecs.DescribeExpressGatewayService(ctx, &ecs.DescribeExpressGatewayServiceInput{ServiceArn: aws.String(arn), Include: []types.ExpressGatewayServiceInclude{types.ExpressGatewayServiceIncludeTags}})
	if err != nil {
		return nil, err
	}
	return out.Service, nil
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
				return fmt.Errorf("Express service became inactive: %s", aws.ToString(service.Status.StatusReason))
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
