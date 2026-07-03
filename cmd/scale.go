package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var (
	scaleCluster string
	scaleDesired int32
)

var scaleCmd = &cobra.Command{
	Use:   "scale [service]",
	Short: "Scale an ECS service to a desired task count",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl scale my-service --desired 3 --cluster my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		fmt.Printf("Scaling service %q to %d tasks...\n", serviceName, scaleDesired)
		if err := client.ScaleService(context.Background(), scaleCluster, serviceName, scaleDesired); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Service %q scaled to %d desired tasks.\n", serviceName, scaleDesired)
		return nil
	},
}

func init() {
	scaleCmd.Flags().StringVar(&scaleCluster, "cluster", "", "ECS cluster name (required)")
	scaleCmd.Flags().Int32Var(&scaleDesired, "desired", 1, "Desired task count")
	scaleCmd.MarkFlagRequired("cluster")
	scaleCmd.MarkFlagRequired("desired")
}
