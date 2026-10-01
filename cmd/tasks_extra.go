package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var taskDefinitionFile, runTaskCluster, runTaskDefinition, runTaskPublicIP, stopTaskCluster, stopTaskARN, stopTaskReason string
var runTaskCount int32
var runTaskLaunchType string
var runTaskSubnets, runTaskSecurityGroups []string

var registerTaskDefinitionCmd = &cobra.Command{Use: "task-definition", Short: "Register an ECS task definition from ECS JSON", Example: "ecsctl register task-definition -f task-definition.json", RunE: func(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(taskDefinitionFile)
	if err != nil {
		return err
	}
	var input ecs.RegisterTaskDefinitionInput
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("parsing ECS task definition JSON: %w", err)
	}
	if input.Family == nil || len(input.ContainerDefinitions) == 0 {
		return fmt.Errorf("task definition JSON must include family and containerDefinitions")
	}
	client, err := aws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	arn, err := client.RegisterTaskDefinition(context.Background(), &input)
	if err != nil {
		return err
	}
	fmt.Printf("Registered task definition %s\n", arn)
	return nil
}}

var registerCmd = &cobra.Command{Use: "register", Short: "Register ECS definitions"}

var runTaskCmd = &cobra.Command{Use: "run-task", Short: "Run one-off ECS tasks", Example: "ecsctl run-task --cluster prod --task-definition batch:4 --subnets subnet-a,subnet-b --security-groups sg-a", RunE: func(cmd *cobra.Command, args []string) error {
	client, err := aws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	arns, err := client.RunTask(context.Background(), aws.RunTaskOptions{Cluster: runTaskCluster, TaskDefinition: runTaskDefinition, Count: runTaskCount, LaunchType: runTaskLaunchType, Subnets: runTaskSubnets, SecurityGroups: runTaskSecurityGroups, AssignPublicIP: runTaskPublicIP})
	if err != nil {
		return err
	}
	if len(arns) == 0 {
		fmt.Println("No tasks started.")
		return nil
	}
	for _, arn := range arns {
		fmt.Println(arn)
	}
	return nil
}}

var stopTaskCmd = &cobra.Command{Use: "stop-task", Short: "Stop an ECS task", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
	client, err := aws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	if err := client.StopTask(context.Background(), stopTaskCluster, stopTaskARN, stopTaskReason); err != nil {
		return err
	}
	fmt.Printf("Stop requested for %s\n", stopTaskARN)
	return nil
}}

func init() {
	registerTaskDefinitionCmd.Flags().StringVarP(&taskDefinitionFile, "file", "f", "", "ECS task definition JSON file (required)")
	_ = registerTaskDefinitionCmd.MarkFlagRequired("file")
	registerCmd.AddCommand(registerTaskDefinitionCmd)
	rootCmd.AddCommand(registerCmd)
	runTaskCmd.Flags().StringVar(&runTaskCluster, "cluster", "", "Cluster name (required)")
	runTaskCmd.Flags().StringVar(&runTaskDefinition, "task-definition", "", "Task definition family:revision or ARN (required)")
	runTaskCmd.Flags().Int32Var(&runTaskCount, "count", 1, "Number of tasks to start")
	runTaskCmd.Flags().StringVar(&runTaskLaunchType, "launch-type", "", "Launch type: FARGATE or EC2 (defaults to the cluster capacity-provider strategy)")
	runTaskCmd.Flags().StringSliceVar(&runTaskSubnets, "subnets", nil, "VPC subnet IDs")
	runTaskCmd.Flags().StringSliceVar(&runTaskSecurityGroups, "security-groups", nil, "VPC security group IDs")
	runTaskCmd.Flags().StringVar(&runTaskPublicIP, "assign-public-ip", "DISABLED", "Assign a public IP: ENABLED or DISABLED")
	_ = runTaskCmd.MarkFlagRequired("cluster")
	_ = runTaskCmd.MarkFlagRequired("task-definition")
	rootCmd.AddCommand(runTaskCmd)
	stopTaskCmd.Flags().StringVar(&stopTaskCluster, "cluster", "", "Cluster name (required)")
	stopTaskCmd.Flags().StringVar(&stopTaskARN, "task", "", "Task ID or ARN (required)")
	stopTaskCmd.Flags().StringVar(&stopTaskReason, "reason", "Stopped by ecsctl", "Reason for stopping the task")
	_ = stopTaskCmd.MarkFlagRequired("cluster")
	_ = stopTaskCmd.MarkFlagRequired("task")
	rootCmd.AddCommand(stopTaskCmd)
}
