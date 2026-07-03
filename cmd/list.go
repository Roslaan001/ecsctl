package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/localconfig"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
)

var listCluster string

// listCmd is the parent for all "list" subcommands.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List ECS resources",
}

// list clusters
var listClustersCmd = &cobra.Command{
	Use:     "clusters",
	Short:   "List all ECS clusters",
	Example: `  ecsctl list clusters`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Try state first
		if resources, ok := listClustersFromState(ctx); ok {
			fmt.Printf("%-40s %-10s %s\n", "NAME", "REGION", "CREATED BY")
			fmt.Println("--------------------------------------------------------------------------------")
			if len(resources) == 0 {
				fmt.Println("No clusters in state.")
				return nil
			}
			for _, r := range resources {
				fmt.Printf("%-40s %-10s %s\n", r.Name, r.Region, r.CreatedBy)
			}
			fmt.Println("\n(source: remote state)")
			return nil
		}

		// Fall back to live AWS
		fmt.Println("(no state configured — querying AWS directly)")
		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.ListClusters(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

// list services
var listServicesCmd = &cobra.Command{
	Use:     "services",
	Short:   "List all services in a cluster",
	Example: `  ecsctl list services --cluster my-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Try state first
		if resources, ok := listServicesFromState(ctx, listCluster); ok {
			fmt.Printf("%-40s %-30s %-10s %s\n", "NAME", "CLUSTER", "REGION", "CREATED BY")
			fmt.Println("--------------------------------------------------------------------------------")
			if len(resources) == 0 {
				fmt.Println("No services in state.")
				return nil
			}
			for _, r := range resources {
				fmt.Printf("%-40s %-30s %-10s %s\n", r.Name, r.Cluster, r.Region, r.CreatedBy)
			}
			fmt.Println("\n(source: remote state)")
			return nil
		}

		// Fall back to live AWS
		fmt.Println("(no state configured — querying AWS directly)")
		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.ListServices(ctx, listCluster); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}

// listClustersFromState returns clusters from remote state.
// Returns (nil, false) if no state is configured.
func listClustersFromState(ctx context.Context) ([]state.Resource, bool) {
	localCfg, err := localconfig.Load()
	if err != nil || localCfg.CurrentContext == "" {
		return nil, false
	}
	_, activeCtx, err := localCfg.GetActiveContext(stateContext)
	if err != nil {
		return nil, false
	}

	backend, err := state.NewBackend(ctx, activeCtx.Bucket, activeCtx.Key, activeCtx.Region, activeCtx.Profile, activeCtx.KmsKeyID)
	if err != nil {
		return nil, false
	}

	st, err := backend.Load(ctx)
	if err != nil {
		return nil, false
	}

	return st.FindClusters(), true
}

// listServicesFromState returns services from remote state, optionally filtered by cluster.
func listServicesFromState(ctx context.Context, cluster string) ([]state.Resource, bool) {
	localCfg, err := localconfig.Load()
	if err != nil || localCfg.CurrentContext == "" {
		return nil, false
	}
	_, activeCtx, err := localCfg.GetActiveContext(stateContext)
	if err != nil {
		return nil, false
	}

	backend, err := state.NewBackend(ctx, activeCtx.Bucket, activeCtx.Key, activeCtx.Region, activeCtx.Profile, activeCtx.KmsKeyID)
	if err != nil {
		return nil, false
	}

	st, err := backend.Load(ctx)
	if err != nil {
		return nil, false
	}

	return st.FindServices(cluster), true
}

func init() {
	listServicesCmd.Flags().StringVar(&listCluster, "cluster", "", "ECS cluster name (required)")
	listServicesCmd.MarkFlagRequired("cluster")

	listCmd.AddCommand(listClustersCmd)
	listCmd.AddCommand(listServicesCmd)
}
