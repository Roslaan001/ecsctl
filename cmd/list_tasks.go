package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var (
	listTasksCluster string
	listTasksService string
)

var listTasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "List running tasks in a cluster or service",
	Example: `  ecsctl list tasks --cluster my-cluster
  ecsctl list tasks --cluster my-cluster --service my-service`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		if err := client.PrintTasks(context.Background(), listTasksCluster, listTasksService); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

func init() {
	listTasksCmd.Flags().StringVar(&listTasksCluster, "cluster", "", "ECS cluster name (required)")
	listTasksCmd.Flags().StringVar(&listTasksService, "service", "", "Filter tasks by service name (optional)")
	_ = listTasksCmd.MarkFlagRequired("cluster")

	listCmd.AddCommand(listTasksCmd)
}
