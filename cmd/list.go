package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/localconfig"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
)

var (
	listCluster       string
	listLive          bool
	listWide          bool
	listName          string
	listStatusFilter  string
	listLaunchType    string
	listDesiredStatus string
	listOutput        string
	listSort          string
	listLimit         int
)

func currentListOptions() aws.ListOptions {
	return aws.ListOptions{Output: strings.ToLower(listOutput), Sort: listSort, Limit: listLimit, Name: listName, Status: listStatusFilter, LaunchType: listLaunchType, DesiredStatus: listDesiredStatus}
}

func validateListOptions(options aws.ListOptions) error {
	if options.Limit < 0 {
		return fmt.Errorf("--limit must be zero or greater")
	}
	if options.Output != "table" && options.Output != "json" {
		return fmt.Errorf("unsupported output format %q (choose table or json)", options.Output)
	}
	return nil
}

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
		options := currentListOptions()
		if err := validateListOptions(options); err != nil {
			return err
		}
		ctx := context.Background()

		// Try state first unless the caller explicitly requests live AWS data.
		if !listLive && listStatusFilter == "" {
			resources, ok := listClustersFromState(ctx)
			if ok {
				rows := make([]map[string]any, 0, len(resources))
				for _, r := range resources {
					var createdAt any
					if !r.CreatedAt.IsZero() {
						createdAt = r.CreatedAt
					}
					rows = append(rows, map[string]any{"name": r.Name, "region": r.Region, "createdBy": r.CreatedBy, "createdAt": createdAt, "arn": r.ARN})
				}
				columns := []aws.ListColumn{{Key: "name", Title: "NAME"}, {Key: "region", Title: "REGION"}, {Key: "createdBy", Title: "CREATED BY"}, {Key: "createdAt", Title: "CREATED AT"}}
				if listWide {
					columns = append(columns, aws.ListColumn{Key: "arn", Title: "ARN"})
				}
				if err := aws.RenderList(rows, columns, options); err != nil {
					return err
				}
				if !strings.EqualFold(listOutput, "json") {
					fmt.Println("\n(source: remote state)")
				}
				return nil
			}
		}

		// Fall back to live AWS
		if !strings.EqualFold(listOutput, "json") {
			fmt.Println("(querying live AWS)")
		}
		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.ListClusters(ctx, options, listWide); err != nil {
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
		options := currentListOptions()
		if err := validateListOptions(options); err != nil {
			return err
		}
		ctx := context.Background()

		// Try state first unless the caller explicitly requests live AWS data.
		if !listLive && listStatusFilter == "" {
			resources, ok := listServicesFromState(ctx, listCluster)
			if ok {
				rows := make([]map[string]any, 0, len(resources))
				for _, r := range resources {
					rows = append(rows, map[string]any{"name": r.Name, "cluster": r.Cluster, "region": r.Region, "createdBy": r.CreatedBy, "arn": r.ARN})
				}
				columns := []aws.ListColumn{{Key: "name", Title: "NAME"}, {Key: "cluster", Title: "CLUSTER"}, {Key: "region", Title: "REGION"}, {Key: "createdBy", Title: "CREATED BY"}}
				if listWide {
					columns = append(columns, aws.ListColumn{Key: "arn", Title: "ARN"})
				}
				if err := aws.RenderList(rows, columns, options); err != nil {
					return err
				}
				if !strings.EqualFold(listOutput, "json") {
					fmt.Println("\n(source: remote state)")
				}
				return nil
			}
		}

		// Fall back to live AWS
		if !strings.EqualFold(listOutput, "json") {
			fmt.Println("(querying live AWS)")
		}
		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.ListServices(ctx, listCluster, options, listWide); err != nil {
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
	listCmd.PersistentFlags().BoolVar(&listLive, "live", false, "Query AWS directly instead of listing the saved state inventory")
	listCmd.PersistentFlags().BoolVar(&listWide, "wide", false, "Include full ARNs in list output")
	listCmd.PersistentFlags().StringVar(&listOutput, "output", "table", "List output format: table or json")
	listCmd.PersistentFlags().StringVar(&listSort, "sort", "", "Sort results by a displayed field")
	listCmd.PersistentFlags().IntVar(&listLimit, "limit", 0, "Limit the number of results (0 means no limit)")
	listServicesCmd.Flags().StringVar(&listCluster, "cluster", "", "ECS cluster name (required)")
	listClustersCmd.Flags().StringVar(&listName, "name", "", "Filter by cluster name prefix")
	listClustersCmd.Flags().StringVar(&listStatusFilter, "status", "", "Filter by cluster status")
	listServicesCmd.Flags().StringVar(&listName, "name", "", "Filter by service name prefix")
	listServicesCmd.Flags().StringVar(&listStatusFilter, "status", "", "Filter by service status")
	_ = listServicesCmd.MarkFlagRequired("cluster")

	listCmd.AddCommand(listClustersCmd)
	listCmd.AddCommand(listServicesCmd)
	listCmd.AddCommand(listTaskDefinitionsCmd)
}
