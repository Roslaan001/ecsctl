//go:build integration

package aws

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

// TestAWSEC2ClusterLifecycle provisions one real EC2-backed ECS cluster and
// deletes it afterward. The explicit gate, profile, and account check prevent
// this test from running against an unintended AWS environment.
func TestAWSEC2ClusterLifecycle(t *testing.T) {
	if os.Getenv("ECSCTL_AWS_EC2_SMOKE") != "1" {
		t.Skip("set ECSCTL_AWS_EC2_SMOKE=1 to run the live AWS EC2 cluster smoke test")
	}
	if profile := strings.TrimSpace(os.Getenv("AWS_PROFILE")); profile != "test" {
		t.Fatalf("AWS_PROFILE must be test for this smoke test, got %q", profile)
	}
	for _, name := range []string{
		"AWS_ENDPOINT_URL", "AWS_ENDPOINT_URL_ECS", "AWS_ENDPOINT_URL_EC2",
		"AWS_ENDPOINT_URL_IAM", "AWS_ENDPOINT_URL_AUTOSCALING", "AWS_ENDPOINT_URL_SSM", "AWS_ENDPOINT_URL_STS",
	} {
		if endpoint := os.Getenv(name); endpoint != "" {
			t.Fatalf("refusing live AWS smoke test with %s=%q", name, endpoint)
		}
	}
	expectedAccount := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EC2_EXPECTED_ACCOUNT"))
	if len(expectedAccount) != 12 {
		t.Fatal("ECSCTL_AWS_EC2_EXPECTED_ACCOUNT must be set to the intended 12-digit AWS account ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithSharedConfigProfile("test"))
	if err != nil {
		t.Fatalf("load AWS profile test: %v", err)
	}
	identity, err := sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("verify AWS caller identity: %v", err)
	}
	if identity.Account == nil || *identity.Account != expectedAccount {
		t.Fatalf("AWS profile test resolved to account %q; expected %q", awssdk.ToString(identity.Account), expectedAccount)
	}
	if awsCfg.Region == "" {
		t.Fatal("AWS profile test has no configured Region")
	}

	client, err := NewECSClient(ctx, awsCfg.Region, "test")
	if err != nil {
		t.Fatalf("create ECS client: %v", err)
	}
	clusterName := fmt.Sprintf("ecsctl-live-smoke-%x", time.Now().UnixNano())
	instanceType := strings.TrimSpace(os.Getenv("ECSCTL_AWS_EC2_INSTANCE_TYPE"))
	if instanceType == "" {
		instanceType = "t3.small"
	}
	if err := client.CreateEC2Cluster(ctx, &ecscfg.ClusterConfig{Name: clusterName}, instanceType, 1); err != nil {
		t.Fatalf("create EC2-backed cluster %q: %v", clusterName, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()
		if err := client.DeleteCluster(cleanupCtx, clusterName, true); err != nil {
			t.Errorf("delete live smoke cluster %q and its EC2 resources: %v", clusterName, err)
		}
	})

	cluster, err := client.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
		Include:  []ecsTypes.ClusterField{ecsTypes.ClusterFieldStatistics},
	})
	if err != nil {
		t.Fatalf("describe smoke cluster: %v", err)
	}
	if len(cluster.Clusters) != 1 {
		t.Fatalf("DescribeClusters returned %d clusters, want 1", len(cluster.Clusters))
	}
	got := cluster.Clusters[0]
	if awssdk.ToString(got.Status) != "ACTIVE" {
		t.Fatalf("cluster status = %q, want ACTIVE", awssdk.ToString(got.Status))
	}
	if got.RegisteredContainerInstancesCount < 1 {
		t.Fatalf("registered container instances = %d, want at least 1", got.RegisteredContainerInstancesCount)
	}
	names := namesForEC2Cluster(clusterName)
	if len(got.DefaultCapacityProviderStrategy) != 1 || awssdk.ToString(got.DefaultCapacityProviderStrategy[0].CapacityProvider) != names.capacityProvider {
		t.Fatalf("default capacity-provider strategy = %#v, want %q", got.DefaultCapacityProviderStrategy, names.capacityProvider)
	}
	providers, err := client.ecs.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{CapacityProviders: []string{names.capacityProvider}})
	if err != nil {
		t.Fatalf("describe EC2 capacity provider: %v", err)
	}
	if len(providers.CapacityProviders) != 1 || string(providers.CapacityProviders[0].Status) != "ACTIVE" {
		t.Fatalf("EC2 capacity provider state = %#v, want one ACTIVE provider", providers.CapacityProviders)
	}
}
