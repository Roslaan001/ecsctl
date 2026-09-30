//go:build integration

package aws

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

// TestAWSExpressServiceLifecycle is an opt-in smoke test for real ECS Express
// Mode. It is intended for the manually dispatched GitHub Actions workflow.
func TestAWSExpressServiceLifecycle(t *testing.T) {
	if os.Getenv("ECSCTL_AWS_EXPRESS_SMOKE") != "1" {
		t.Skip("set ECSCTL_AWS_EXPRESS_SMOKE=1 to run against AWS")
	}
	for _, name := range []string{"AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL_ECS"} {
		if endpoint := os.Getenv(name); endpoint != "" {
			t.Fatalf("refusing AWS Express smoke test with %s=%q", name, endpoint)
		}
	}

	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_DEFAULT_REGION"))
	}
	cluster := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EXPRESS_CLUSTER"))
	infrastructureRole := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EXPRESS_INFRASTRUCTURE_ROLE_ARN"))
	executionRole := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EXPRESS_EXECUTION_ROLE_ARN"))
	image := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EXPRESS_IMAGE"))
	for name, value := range map[string]string{
		"AWS region": region, "ECSCTL_AWS_EXPRESS_CLUSTER": cluster,
		"ECSCTL_AWS_EXPRESS_INFRASTRUCTURE_ROLE_ARN": infrastructureRole,
		"ECSCTL_AWS_EXPRESS_EXECUTION_ROLE_ARN":      executionRole,
		"ECSCTL_AWS_EXPRESS_IMAGE":                   image,
	} {
		if value == "" {
			t.Fatalf("%s is required for the AWS Express smoke test", name)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	client, err := NewECSClient(ctx, region, "")
	if err != nil {
		t.Fatalf("create ECS client: %v", err)
	}

	serviceName := fmt.Sprintf("ecsctl-smoke-%x", time.Now().UnixNano())
	cfg := &ecscfg.ExpressServiceConfig{
		ServiceName:           serviceName,
		Cluster:               cluster,
		InfrastructureRoleARN: infrastructureRole,
		ExecutionRoleARN:      executionRole,
		Image:                 image,
		ContainerPort:         80,
		MinTaskCount:          1,
		MaxTaskCount:          1,
	}
	if port := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EXPRESS_CONTAINER_PORT")); port != "" {
		parsed, err := strconv.ParseInt(port, 10, 32)
		if err != nil || parsed < 1 || parsed > 65535 {
			t.Fatalf("invalid ECSCTL_AWS_EXPRESS_CONTAINER_PORT %q", port)
		}
		cfg.ContainerPort = int32(parsed)
	}
	cfg.Subnets = splitCSV(os.Getenv("ECSCTL_AWS_EXPRESS_SUBNETS"))
	cfg.SecurityGroups = splitCSV(os.Getenv("ECSCTL_AWS_EXPRESS_SECURITY_GROUPS"))

	arn, err := client.CreateExpressService(ctx, cfg)
	if err != nil {
		t.Fatalf("create Express service: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		if err := client.DeleteExpressService(cleanupCtx, arn); err != nil {
			t.Errorf("request Express service deletion for %q: %v", arn, err)
		}
	})

	if err := client.WaitForExpressService(ctx, arn); err != nil {
		t.Fatalf("wait for Express service: %v", err)
	}
	service, err := client.DescribeExpressService(ctx, arn)
	if err != nil {
		t.Fatalf("describe Express service: %v", err)
	}
	if service == nil || service.ServiceArn == nil || *service.ServiceArn != arn {
		t.Fatalf("DescribeExpressService returned unexpected service: %#v", service)
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
