package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var (
	execCluster   string
	execContainer string
	execCommand   string
	execTask      string
)

var execCmd = &cobra.Command{
	Use:   "exec [service]",
	Short: "Execute a command in a running ECS task container",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl exec my-service --cluster my-cluster --container web
  ecsctl exec my-service --cluster my-cluster --container web --command "/bin/sh"
  ecsctl exec my-service --cluster my-cluster --task abc123def456`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		opts := aws.ExecOptions{
			Cluster:   execCluster,
			Service:   serviceName,
			TaskID:    execTask,
			Container: execContainer,
			Command:   execCommand,
		}

		fmt.Printf("Opening shell in service %q (cluster: %s)...\n", serviceName, execCluster)
		if err := client.ExecInTask(context.Background(), opts); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		return nil
	},
}

func init() {
	execCmd.Flags().StringVar(&execCluster, "cluster", "", "ECS cluster name (required)")
	execCmd.Flags().StringVar(&execContainer, "container", "", "Container name to exec into (defaults to first)")
	execCmd.Flags().StringVar(&execCommand, "command", "/bin/sh", "Command to run inside the container")
	execCmd.Flags().StringVar(&execTask, "task", "", "Specific task ID (defaults to a running task in the service)")
	_ = execCmd.MarkFlagRequired("cluster")
}
