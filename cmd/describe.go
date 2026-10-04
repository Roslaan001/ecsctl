package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var describeClusterName string
var describeTaskCluster string

// describeCmd is the parent for all "describe" subcommands.
var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Show detailed information about an ECS resource",
}

// describe cluster
var describeClusterCmd = &cobra.Command{
	Use:   "cluster [name]",
	Short: "Show detailed information about an ECS cluster",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl describe cluster my-cluster
  ecsctl describe cluster my-cluster --region eu-west-2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		if err := client.PrintClusterDetail(context.Background(), clusterName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

// describe service
var describeServiceCmd = &cobra.Command{
	Use:   "service [name]",
	Short: "Show detailed information about an ECS service",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl describe service my-service --cluster my-cluster
  ecsctl describe service my-service --cluster my-cluster --region eu-west-2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		if err := client.PrintServiceDetail(context.Background(), describeClusterName, serviceName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

var describeTaskCmd = &cobra.Command{
	Use:     "task [task-id-or-arn]",
	Short:   "Show task status, container details, and stop reasons",
	Args:    cobra.ExactArgs(1),
	Example: "  ecsctl describe task 0123456789abcdef0 --cluster production",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.PrintTaskDetail(context.Background(), describeTaskCluster, args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	describeServiceCmd.Flags().StringVar(&describeClusterName, "cluster", "", "ECS cluster name (required)")
	_ = describeServiceCmd.MarkFlagRequired("cluster")
	describeTaskCmd.Flags().StringVar(&describeTaskCluster, "cluster", "", "ECS cluster name (required)")
	_ = describeTaskCmd.MarkFlagRequired("cluster")

	describeCmd.AddCommand(describeClusterCmd)
	describeCmd.AddCommand(describeServiceCmd)
	describeCmd.AddCommand(describeTaskCmd)
}
