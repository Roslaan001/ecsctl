//go:build integration

package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
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
	expectedAccount := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EXPRESS_EXPECTED_ACCOUNT"))
	for name, value := range map[string]string{
		"AWS region":                                 region,
		"ECSCTL_AWS_EXPRESS_EXPECTED_ACCOUNT":        expectedAccount,
		"ECSCTL_AWS_EXPRESS_INFRASTRUCTURE_ROLE_ARN": infrastructureRole,
		"ECSCTL_AWS_EXPRESS_EXECUTION_ROLE_ARN":      executionRole,
	} {
		if value == "" {
			t.Fatalf("%s is required for the AWS Express smoke test", name)
		}
	}
	if image == "" {
		image = "public.ecr.aws/docker/library/nginx:stable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		t.Fatalf("load AWS credentials: %v", err)
	}
	identity, err := sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("verify AWS caller identity: %v", err)
	}
	if identity.Account == nil || *identity.Account != expectedAccount {
		t.Fatalf("AWS credentials resolved to account %q; expected %q", awssdk.ToString(identity.Account), expectedAccount)
	}
	client, err := NewECSClient(ctx, region, "")
	if err != nil {
		t.Fatalf("create ECS client: %v", err)
	}

	serviceName := fmt.Sprintf("ecsctl-smoke-%x", time.Now().UnixNano())
	if cluster == "" {
		cluster = fmt.Sprintf("ecsctl-express-smoke-%x", time.Now().UnixNano())
		if err := client.CreateCluster(ctx, &ecscfg.ClusterConfig{Name: cluster}); err != nil {
			t.Fatalf("create temporary ECS cluster %q: %v", cluster, err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cleanupCancel()
			if err := client.DeleteCluster(cleanupCtx, cluster, false); err != nil {
				t.Errorf("delete temporary ECS cluster %q: %v", cluster, err)
			}
		})
	}
	cfg := &ecscfg.ExpressServiceConfig{
		ServiceName:           serviceName,
		Cluster:               cluster,
		InfrastructureRoleARN: infrastructureRole,
		ExecutionRoleARN:      executionRole,
		Image:                 image,
		ContainerPort:         80,
		HealthCheckPath:       "/",
		MinTaskCount:          1,
		MaxTaskCount:          1,
		Tags:                  map[string]string{"ecsctl:smoke-test": serviceName},
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
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cleanupCancel()
		if err := client.DeleteExpressService(cleanupCtx, arn); err != nil {
			t.Errorf("request Express service deletion for %q: %v", arn, err)
			return
		}
		if err := waitForExpressServiceDeletion(cleanupCtx, client, arn); err != nil {
			t.Errorf("wait for Express service %q cleanup: %v", arn, err)
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

func waitForExpressServiceDeletion(ctx context.Context, client *Client, arn string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		service, err := client.DescribeExpressService(ctx, arn)
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && strings.Contains(apiErr.ErrorCode(), "NotFound") {
				return nil
			}
			return err
		}
		if service == nil || (service.Status != nil && service.Status.StatusCode == "INACTIVE") {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for Express service deletion")
		case <-ticker.C:
		}
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
