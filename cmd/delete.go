package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
)

var (
	deleteCluster string
	deleteForce   bool
)

// deleteCmd is the parent for all "delete" subcommands.
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete ECS resources",
}

// delete cluster
var deleteClusterCmd = &cobra.Command{
	Use:     "cluster [name]",
	Short:   "Delete an ECS cluster",
	Args:    cobra.ExactArgs(1),
	Example: `  ecsctl delete cluster my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		clusterName := args[0]
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
		var clusterARN string
		if session != nil {
			clusterARN, err = client.ClusterARN(context.Background(), clusterName)
			if err != nil {
				return err
			}
		}

		fmt.Printf("Deleting cluster %q...\n", clusterName)
		if err := client.DeleteCluster(context.Background(), clusterName, deleteForce); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Cluster %q deleted.\n", clusterName)
		if session != nil {
			if err := session.Update(func(st *state.State) {
				st.RemoveClusterAndServicesByARN(clusterName, clusterARN, client.Region())
			}); err != nil {
				return fmt.Errorf("cluster deleted but remote state was not updated: %w", err)
			}
		}
		return nil
	},
}

// delete service
var deleteServiceCmd = &cobra.Command{
	Use:     "service [name]",
	Short:   "Delete an ECS service",
	Args:    cobra.ExactArgs(1),
	Example: `  ecsctl delete service my-service --cluster my-cluster`,
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
		var serviceARN string
		if session != nil {
			serviceARN, err = client.ServiceARN(context.Background(), deleteCluster, serviceName)
			if err != nil {
				return err
			}
		}

		fmt.Printf("Deleting service %q from cluster %q...\n", serviceName, deleteCluster)
		if err := client.DeleteService(context.Background(), deleteCluster, serviceName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Service %q deleted.\n", serviceName)
		if session != nil {
			if err := session.Update(func(st *state.State) { st.RemoveResourceByARN(serviceARN) }); err != nil {
				return fmt.Errorf("service deleted but remote state was not updated: %w", err)
			}
		}
		return nil
	},
}

func init() {
	deleteServiceCmd.Flags().StringVar(&deleteCluster, "cluster", "", "ECS cluster name (required)")
	_ = deleteServiceCmd.MarkFlagRequired("cluster")

	deleteClusterCmd.Flags().BoolVar(&deleteForce, "force", false, "Delete all services in the cluster before deleting the cluster")

	deleteCmd.AddCommand(deleteClusterCmd)
	deleteCmd.AddCommand(deleteServiceCmd)
}
