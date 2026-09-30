//go:build integration

package aws

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

// TestFlociECSServiceLifecycle exercises ecsctl's ECS client against a local
// Floci endpoint. Set AWS_ENDPOINT_URL to the Floci endpoint to opt in.
// The test only creates local resources and uses dummy credentials.
func TestFlociECSServiceLifecycle(t *testing.T) {
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		t.Skip("set AWS_ENDPOINT_URL to run the Floci integration test")
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Hostname() == "" {
		t.Fatalf("invalid Floci endpoint %q", endpoint)
	}
	host := parsedEndpoint.Hostname()
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("Floci integration tests only allow a loopback endpoint, got %q", endpoint)
	}
	t.Setenv("AWS_ENDPOINT_URL_ECS", endpoint)

	// Do not allow a developer's configured AWS credentials to be used.
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = "us-east-1"
		t.Setenv("AWS_DEFAULT_REGION", region)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := NewECSClient(ctx, region, "")
	if err != nil {
		t.Fatalf("create ECS client: %v", err)
	}

	// Wait for the emulator to accept requests before creating test resources.
	for {
		_, err = client.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{})
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Floci did not become ready: %v (last request error: %v)", ctx.Err(), err)
		case <-time.After(500 * time.Millisecond):
		}
	}

	suffix := fmt.Sprintf("%x", time.Now().UnixNano())
	cluster := "ecsctl-floci-" + suffix
	serviceName := "smoke-" + suffix[:min(12, len(suffix))]
	family := "ecsctl-floci-" + suffix

	if err := client.CreateCluster(ctx, &ecscfg.ClusterConfig{Name: cluster}); err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := client.DeleteCluster(cleanupCtx, cluster, true); err != nil {
			t.Errorf("delete cluster %q: %v", cluster, err)
		}
	})

	taskDefinitionARN, err := client.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:      awssdk.String(family),
		NetworkMode: types.NetworkModeBridge,
		ContainerDefinitions: []types.ContainerDefinition{{
			Name:      awssdk.String("app"),
			Image:     awssdk.String("public.ecr.aws/docker/library/nginx:latest"),
			Essential: awssdk.Bool(true),
		}},
	})
	if err != nil {
		t.Fatalf("register task definition: %v", err)
	}
	if err := client.CreateService(ctx, &ecscfg.ServiceConfig{
		Name:           serviceName,
		Cluster:        cluster,
		TaskDefinition: taskDefinitionARN,
		LaunchType:     string(types.LaunchTypeEc2),
		DesiredCount:   0,
	}); err != nil {
		t.Fatalf("create service: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := client.DeleteService(cleanupCtx, cluster, serviceName); err != nil {
			t.Errorf("delete service %q: %v", serviceName, err)
		}
	})

	services, err := client.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  awssdk.String(cluster),
		Services: []string{serviceName},
	})
	if err != nil {
		t.Fatalf("describe service: %v", err)
	}
	if len(services.Services) != 1 {
		t.Fatalf("DescribeServices returned %d services, want 1 (failures: %#v)", len(services.Services), services.Failures)
	}
	if got := awssdk.ToString(services.Services[0].Status); got != "ACTIVE" {
		t.Fatalf("service status = %q, want ACTIVE", got)
	}
}
