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
	listStateOnly     bool
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

		resources, hasState := listClustersFromState(ctx)
		if listStateOnly {
			if listLive {
				return fmt.Errorf("--state and --live cannot be used together")
			}
			if !hasState {
				return fmt.Errorf("no active remote state inventory is available; configure a context with 'ecsctl state init'")
			}
			if listStatusFilter != "" {
				return fmt.Errorf("--status requires live AWS data; omit --state")
			}
			if err := aws.RenderList(clusterStateRows(resources), aws.ClusterListColumns(listWide), options); err != nil {
				return err
			}
			if !strings.EqualFold(listOutput, "json") {
				fmt.Println("\n(source: remote state; live AWS fields are unavailable)")
			}
			return nil
		}

		// Fall back to live AWS
		options.Metadata = listMetadata(resources)
		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.ListClusters(ctx, options, listWide); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		if !strings.EqualFold(listOutput, "json") {
			fmt.Println("\n(source: live AWS; tracking metadata from remote state where available)")
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

		resources, hasState := listServicesFromState(ctx, listCluster)
		if listStateOnly {
			if listLive {
				return fmt.Errorf("--state and --live cannot be used together")
			}
			if !hasState {
				return fmt.Errorf("no active remote state inventory is available; configure a context with 'ecsctl state init'")
			}
			if listStatusFilter != "" {
				return fmt.Errorf("--status requires live AWS data; omit --state")
			}
			if err := aws.RenderList(serviceStateRows(resources), aws.ServiceListColumns(listWide), options); err != nil {
				return err
			}
			if !strings.EqualFold(listOutput, "json") {
				fmt.Println("\n(source: remote state; live AWS fields are unavailable)")
			}
			return nil
		}

		// Fall back to live AWS
		options.Metadata = listMetadata(resources)
		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if err := client.ListServices(ctx, listCluster, options, listWide); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		if !strings.EqualFold(listOutput, "json") {
			fmt.Println("\n(source: live AWS; tracking metadata from remote state where available)")
		}
		return nil
	},
}

// listClustersFromState returns clusters from remote state.
// Returns (nil, false) if no state is configured.
func listClustersFromState(ctx context.Context) ([]state.Resource, bool) {
	localCfg, err := localconfig.Load()
	if err != nil || (localCfg.CurrentContext == "" && stateContext == "") {
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
	if err != nil || (localCfg.CurrentContext == "" && stateContext == "") {
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

func listMetadata(resources []state.Resource) map[string]aws.ResourceMetadata {
	metadata := make(map[string]aws.ResourceMetadata, len(resources))
	for _, resource := range resources {
		if resource.ARN == "" {
			continue
		}
		metadata[resource.ARN] = aws.ResourceMetadata{CreatedBy: resource.CreatedBy, CreatedAt: resource.CreatedAt}
	}
	return metadata
}

func clusterStateRows(resources []state.Resource) []map[string]any {
	rows := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		var trackedAt any
		if !resource.CreatedAt.IsZero() {
			trackedAt = resource.CreatedAt
		}
		rows = append(rows, map[string]any{"name": resource.Name, "status": nil, "services": nil, "running": nil, "pending": nil, "capacityProviders": nil, "region": resource.Region, "createdBy": resource.CreatedBy, "trackedAt": trackedAt, "arn": resource.ARN})
	}
	return rows
}

func serviceStateRows(resources []state.Resource) []map[string]any {
	rows := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		var trackedAt any
		if !resource.CreatedAt.IsZero() {
			trackedAt = resource.CreatedAt
		}
		rows = append(rows, map[string]any{"name": resource.Name, "status": nil, "health": nil, "deployment": nil, "desired": nil, "running": nil, "pending": nil, "launchType": nil, "taskDefinition": nil, "createdAt": nil, "cluster": resource.Cluster, "region": resource.Region, "createdBy": resource.CreatedBy, "trackedAt": trackedAt, "arn": resource.ARN, "taskDefinitionArn": nil})
	}
	return rows
}

func init() {
	listCmd.PersistentFlags().BoolVar(&listLive, "live", false, "Explicitly query AWS (the default; use --state for saved inventory)")
	listCmd.PersistentFlags().BoolVar(&listWide, "wide", false, "Include full ARNs in list output")
	listCmd.PersistentFlags().StringVar(&listOutput, "output", "table", "List output format: table or json")
	listCmd.PersistentFlags().StringVar(&listSort, "sort", "", "Sort results by a displayed field")
	listCmd.PersistentFlags().IntVar(&listLimit, "limit", 0, "Limit the number of results (0 means no limit)")
	listServicesCmd.Flags().StringVar(&listCluster, "cluster", "", "ECS cluster name (required)")
	listClustersCmd.Flags().StringVar(&listName, "name", "", "Filter by cluster name prefix")
	listClustersCmd.Flags().StringVar(&listStatusFilter, "status", "", "Filter by cluster status")
	listServicesCmd.Flags().StringVar(&listName, "name", "", "Filter by service name prefix")
	listServicesCmd.Flags().StringVar(&listStatusFilter, "status", "", "Filter by service status")
	listClustersCmd.Flags().BoolVar(&listStateOnly, "state", false, "List the saved state inventory instead of querying AWS")
	listServicesCmd.Flags().BoolVar(&listStateOnly, "state", false, "List the saved state inventory instead of querying AWS")
	_ = listServicesCmd.MarkFlagRequired("cluster")

	listCmd.AddCommand(listClustersCmd)
	listCmd.AddCommand(listServicesCmd)
	listCmd.AddCommand(listTaskDefinitionsCmd)
}
