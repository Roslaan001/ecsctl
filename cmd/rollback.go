package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/config"
	"github.com/spf13/cobra"
)

var (
	rollbackCluster string
	rollbackWait    bool
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [service]",
	Short: "Roll back a service to its previous completed deployment",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl rollback my-service --cluster my-cluster
  ecsctl rollback my-service --cluster my-cluster --wait`,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		serviceName := args[0]
		session, err := beginStateSession(context.Background())
		if err != nil {
			return fmt.Errorf("locking remote state: %w", err)
		}
		if session != nil {
			defer closeStateSessionOnReturn(session, &runErr, "releasing remote state lock")
		}

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		taskDefinition, err := client.RollbackService(context.Background(), rollbackCluster, serviceName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		if session != nil {
			if err := updateTrackedServiceConfig(session, client, rollbackCluster, serviceName, func(cfg *config.ServiceConfig) {
				cfg.TaskDefinition = taskDefinition
			}); err != nil {
				return fmt.Errorf("rollback started but tracked state was not updated: %w", err)
			}
		}

		fmt.Printf("✓ Rollback started for service %q using task definition %s.\n", serviceName, taskDefinition)
		if rollbackWait {
			fmt.Println("Waiting for rollback deployment to complete...")
			if err := client.WaitForDeployment(context.Background(), rollbackCluster, serviceName, taskDefinition); err != nil {
				fmt.Fprintf(os.Stderr, "Rollback failed: %v\n", err)
				return err
			}
			fmt.Printf("✓ Rollback complete. Service %q is stable.\n", serviceName)
		} else {
			fmt.Println("Tip: use --wait to poll until the rollback is complete.")
		}
		return nil
	},
}

func init() {
	rollbackCmd.Flags().StringVar(&rollbackCluster, "cluster", "", "ECS cluster name (required)")
	rollbackCmd.Flags().BoolVar(&rollbackWait, "wait", false, "Wait for the rollback deployment to complete")
	_ = rollbackCmd.MarkFlagRequired("cluster")
	rootCmd.AddCommand(rollbackCmd)
}
