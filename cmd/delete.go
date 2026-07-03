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
	Use:   "cluster [name]",
	Short: "Delete an ECS cluster",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl delete cluster my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		fmt.Printf("Deleting cluster %q...\n", clusterName)
		if err := client.DeleteCluster(context.Background(), clusterName, deleteForce); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Cluster %q deleted.\n", clusterName)
		if err := removeClusterFromState(clusterName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cluster deleted but state not updated: %v\n", err)
		}
		return nil
	},
}

// delete service
var deleteServiceCmd = &cobra.Command{
	Use:   "service [name]",
	Short: "Delete an ECS service",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl delete service my-service --cluster my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		fmt.Printf("Deleting service %q from cluster %q...\n", serviceName, deleteCluster)
		if err := client.DeleteService(context.Background(), deleteCluster, serviceName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Service %q deleted.\n", serviceName)
		if err := removeFromState(state.ResourceTypeService, serviceName, deleteCluster); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: service deleted but state not updated: %v\n", err)
		}
		return nil
	},
}

func init() {
	deleteServiceCmd.Flags().StringVar(&deleteCluster, "cluster", "", "ECS cluster name (required)")
	deleteServiceCmd.MarkFlagRequired("cluster")

	deleteClusterCmd.Flags().BoolVar(&deleteForce, "force", false, "Delete all services in the cluster before deleting the cluster")

	deleteCmd.AddCommand(deleteClusterCmd)
	deleteCmd.AddCommand(deleteServiceCmd)
}

// removeFromState removes a resource from remote state after deletion.
// Silently skips if no state context is configured.
func removeFromState(resourceType state.ResourceType, name, cluster string) error {
	return writeState(func(st *state.State) {
		st.RemoveResource(resourceType, name, cluster)
	})
}

// removeClusterFromState removes the cluster and all services inside it from remote state.
func removeClusterFromState(clusterName string) error {
	return writeState(func(st *state.State) {
		st.RemoveClusterAndServices(clusterName)
	})
}
