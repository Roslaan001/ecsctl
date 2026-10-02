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

// TestAWSFargateTaskLifecycle launches a short-lived Fargate task in a
// temporary cluster and removes the task definition and cluster afterward.
func TestAWSFargateTaskLifecycle(t *testing.T) {
	if os.Getenv("ECSCTL_AWS_FARGATE_SMOKE") != "1" {
		t.Skip("set ECSCTL_AWS_FARGATE_SMOKE=1 to run the live AWS Fargate task smoke test")
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
	expectedAccount := strings.TrimSpace(os.Getenv("ECSCTL_AWS_FARGATE_EXPECTED_ACCOUNT"))
	if len(expectedAccount) != 12 {
		t.Fatal("ECSCTL_AWS_FARGATE_EXPECTED_ACCOUNT must be set to the intended 12-digit AWS account ID")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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
	ecsClient := ecs.NewFromConfig(awsCfg)
	_, subnetIDs, securityGroupID, err := client.defaultVPCNetwork(ctx)
	if err != nil {
		t.Fatalf("find default VPC public subnets for Fargate task: %v", err)
	}

	clusterName := fmt.Sprintf("ecsctl-fargate-smoke-%x", time.Now().UnixNano())
	if err := client.CreateCluster(ctx, &ecscfg.ClusterConfig{Name: clusterName}); err != nil {
		t.Fatalf("create temporary Fargate cluster %q: %v", clusterName, err)
	}
	var taskARN, taskDefinitionARN string
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()
		if taskARN != "" {
			if err := stopAndWaitForFargateTask(cleanupCtx, client, clusterName, taskARN); err != nil {
				t.Errorf("stop live Fargate task %q: %v", taskARN, err)
			}
		}
		if taskDefinitionARN != "" {
			if _, err := ecsClient.DeregisterTaskDefinition(cleanupCtx, &ecs.DeregisterTaskDefinitionInput{TaskDefinition: awssdk.String(taskDefinitionARN)}); err != nil {
				t.Errorf("deregister live Fargate task definition %q: %v", taskDefinitionARN, err)
			}
		}
		if err := client.DeleteCluster(cleanupCtx, clusterName, true); err != nil {
			t.Errorf("delete temporary Fargate cluster %q: %v", clusterName, err)
		}
	})

	image := strings.TrimSpace(os.Getenv("ECSCTL_AWS_FARGATE_IMAGE"))
	if image == "" {
		image = "public.ecr.aws/docker/library/busybox:1.36"
	}
	definition := &ecs.RegisterTaskDefinitionInput{
		Family:                  awssdk.String(clusterName),
		NetworkMode:             ecsTypes.NetworkModeAwsvpc,
		RequiresCompatibilities: []ecsTypes.Compatibility{ecsTypes.CompatibilityFargate},
		Cpu:                     awssdk.String("256"),
		Memory:                  awssdk.String("512"),
		ContainerDefinitions: []ecsTypes.ContainerDefinition{{
			Name: awssdk.String("smoke"), Image: awssdk.String(image), Essential: awssdk.Bool(true),
			Command: []string{"sh", "-c", "echo ecsctl-fargate-smoke"},
		}},
	}
	if roleARN := strings.TrimSpace(os.Getenv("ECSCTL_AWS_FARGATE_EXECUTION_ROLE_ARN")); roleARN != "" {
		definition.ExecutionRoleArn = awssdk.String(roleARN)
	}
	registered, err := client.ecs.RegisterTaskDefinition(ctx, definition)
	if err != nil {
		t.Fatalf("register Fargate smoke task definition: %v", err)
	}
	taskDefinitionARN = awssdk.ToString(registered.TaskDefinition.TaskDefinitionArn)
	if taskDefinitionARN == "" {
		t.Fatal("RegisterTaskDefinition returned no task definition ARN")
	}

	run, err := client.ecs.RunTask(ctx, &ecs.RunTaskInput{
		Cluster: awssdk.String(clusterName), TaskDefinition: awssdk.String(taskDefinitionARN),
		LaunchType: ecsTypes.LaunchTypeFargate, Count: awssdk.Int32(1),
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
			Subnets: subnetIDs, SecurityGroups: []string{securityGroupID}, AssignPublicIp: ecsTypes.AssignPublicIpEnabled,
		}},
	})
	if err != nil {
		t.Fatalf("run Fargate smoke task: %v", err)
	}
	if len(run.Failures) != 0 {
		t.Fatalf("RunTask returned failures: %#v", run.Failures)
	}
	if len(run.Tasks) != 1 || awssdk.ToString(run.Tasks[0].TaskArn) == "" {
		t.Fatalf("RunTask returned %d tasks, want exactly one: %#v", len(run.Tasks), run.Tasks)
	}
	taskARN = awssdk.ToString(run.Tasks[0].TaskArn)

	task, err := waitForFargateTask(ctx, client, clusterName, taskARN)
	if err != nil {
		t.Fatalf("wait for Fargate task to stop: %v", err)
	}
	if string(task.LaunchType) != string(ecsTypes.LaunchTypeFargate) {
		t.Fatalf("task launch type = %q, want FARGATE", task.LaunchType)
	}
	if len(task.Containers) == 0 || task.Containers[0].ExitCode == nil || *task.Containers[0].ExitCode != 0 {
		t.Fatalf("Fargate smoke container did not exit successfully: %#v", task.Containers)
	}
}

func waitForFargateTask(ctx context.Context, client *Client, clusterName, taskARN string) (ecsTypes.Task, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		out, err := client.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: awssdk.String(clusterName), Tasks: []string{taskARN}})
		if err != nil {
			return ecsTypes.Task{}, fmt.Errorf("describing Fargate task: %w", err)
		}
		if len(out.Failures) > 0 {
			return ecsTypes.Task{}, fmt.Errorf("describing Fargate task returned failures: %#v", out.Failures)
		}
		if len(out.Tasks) == 1 && awssdk.ToString(out.Tasks[0].LastStatus) == "STOPPED" {
			return out.Tasks[0], nil
		}
		select {
		case <-ctx.Done():
			return ecsTypes.Task{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func stopAndWaitForFargateTask(ctx context.Context, client *Client, clusterName, taskARN string) error {
	described, err := client.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: awssdk.String(clusterName), Tasks: []string{taskARN}})
	if err != nil {
		return fmt.Errorf("checking Fargate task before cleanup: %w", err)
	}
	if len(described.Tasks) == 1 && awssdk.ToString(described.Tasks[0].LastStatus) == "STOPPED" {
		return nil
	}
	if _, err := client.ecs.StopTask(ctx, &ecs.StopTaskInput{Cluster: awssdk.String(clusterName), Task: awssdk.String(taskARN), Reason: awssdk.String("ecsctl smoke test cleanup")}); err != nil {
		return fmt.Errorf("requesting Fargate task stop: %w", err)
	}
	_, err = waitForFargateTask(ctx, client, clusterName, taskARN)
	return err
}
