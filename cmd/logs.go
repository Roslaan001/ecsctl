package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var (
	logsCluster   string
	logsContainer string
	logsTail      int32
	logsFollow    bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Fetch logs from an ECS service",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl logs my-service --cluster my-cluster
  ecsctl logs my-service --cluster my-cluster --tail 100 --follow`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		opts := aws.LogsOptions{
			Cluster:       logsCluster,
			Service:       serviceName,
			ContainerName: logsContainer,
			Tail:          logsFollow,
			TailLines:     true, // always respect --tail
		}

		if err := client.FetchLogs(context.Background(), opts, int(logsTail)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().StringVar(&logsCluster, "cluster", "", "ECS cluster name (required)")
	logsCmd.Flags().StringVar(&logsContainer, "container", "", "Container name (defaults to first container)")
	logsCmd.Flags().Int32Var(&logsTail, "tail", 50, "Number of recent log lines to show")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Stream logs continuously")
	_ = logsCmd.MarkFlagRequired("cluster")
}
