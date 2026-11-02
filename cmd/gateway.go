package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/config"
	"github.com/roslaan001/ecsctl/pkg/localconfig"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
)

var (
	gatewayFile               string
	gatewayName               string
	gatewayInfraRole          string
	gatewayScheme             string
	gatewaySubnets            []string
	gatewaySecurityGroups     []string
	gatewayMaxALBs            int32
	gatewayWebACLArn          string
	gatewayDeletionProtection bool
	gatewayClusters           []string
	gatewayTags               []string
	gatewayShowResources      bool

	assocGateway string
	assocCluster string
)

// gatewayCmd is the parent for the dedicated 'ecsctl gateway' command group.
var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Manage Amazon ECS Gateways",
	Long:  "Create, inspect, list, delete, and configure Amazon ECS Gateways for managed ingress networking.",
}

// createGatewayCmd creates an ECS Gateway from a config file or inline flags.
var createGatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Create an Amazon ECS Gateway",
	Example: `  # From a YAML file
  ecsctl create gateway -f gateway.yaml

  # Inline flags (role is auto-created if omitted)
  ecsctl create gateway --name my-gateway \
    --subnets subnet-12345,subnet-67890 \
    --clusters prod-cluster`,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		var cfg *config.GatewayConfig
		var err error

		if gatewayFile != "" {
			cfg, err = config.LoadGatewayConfig(gatewayFile)
			if err != nil {
				return fmt.Errorf("loading gateway config: %w", err)
			}
		} else {
			if gatewayName == "" {
				return fmt.Errorf("--name is required (or use -f to provide a config file)")
			}
			scheme := gatewayScheme
			if scheme == "" {
				scheme = "internet-facing"
			}
			delProt := gatewayDeletionProtection
			cfg = &config.GatewayConfig{
				Name:                  gatewayName,
				InfrastructureRoleARN: gatewayInfraRole,
				DeletionProtection:    &delProt,
				Clusters:              gatewayClusters,
				Tags:                  parseTags(gatewayTags),
				Configuration: &config.GatewayConfigurationConfig{
					HostSharedAlb: &config.HostSharedAlbConfig{
						Scheme:      scheme,
						MaxAlbCount: gatewayMaxALBs,
						WebACLArn:   gatewayWebACLArn,
					},
				},
			}
			if len(gatewaySubnets) > 0 || len(gatewaySecurityGroups) > 0 {
				var inbSG string
				if len(gatewaySecurityGroups) > 0 {
					inbSG = gatewaySecurityGroups[0]
				}
				cfg.Configuration.HostSharedAlb.Network = &config.GatewayNetworkConfig{
					Subnets:              gatewaySubnets,
					InboundSecurityGroup: inbSG,
				}
			}
		}

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

		fmt.Printf("Creating gateway %q...\n", cfg.Name)
		arn, err := client.CreateGateway(context.Background(), cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Gateway %q created successfully (%s).\n", cfg.Name, arn)
		if session != nil {
			if err := recordGatewayState(session, client, cfg, arn, region); err != nil {
				return fmt.Errorf("gateway created but remote state was not updated: %w", err)
			}
		}
		return nil
	},
}

// describeGatewayCmd describes an ECS Gateway and its routes.
var describeGatewayCmd = &cobra.Command{
	Use:   "gateway [name-or-arn]",
	Short: "Show detailed information about an ECS Gateway",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl describe gateway my-gateway
  ecsctl describe gateway my-gateway --resources`,
	RunE: func(cmd *cobra.Command, args []string) error {
		nameOrArn := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		return client.PrintGatewayDetail(context.Background(), nameOrArn, gatewayShowResources)
	},
}

// deleteGatewayCmd deletes an ECS Gateway.
var deleteGatewayCmd = &cobra.Command{
	Use:     "gateway [name-or-arn]",
	Short:   "Delete an ECS Gateway",
	Args:    cobra.ExactArgs(1),
	Example: `  ecsctl delete gateway my-gateway`,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		nameOrArn := args[0]

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

		fmt.Printf("Deleting gateway %q...\n", nameOrArn)
		if err := client.DeleteGateway(context.Background(), nameOrArn); err != nil {
			return err
		}

		if session != nil {
			if err := session.Update(func(st *state.State) {
				st.RemoveResource(state.ResourceTypeGateway, nameOrArn, "")
			}); err != nil {
				return fmt.Errorf("updating remote state: %w", err)
			}
		}

		fmt.Printf("✓ Gateway %q deleted.\n", nameOrArn)
		return nil
	},
}

// listGatewaysCmd lists ECS Gateways.
var listGatewaysCmd = &cobra.Command{
	Use:   "gateways",
	Short: "List ECS Gateways",
	Example: `  ecsctl list gateways
  ecsctl list gateways --output json
  ecsctl list gateways --state`,
	RunE: func(cmd *cobra.Command, args []string) error {
		options := currentListOptions()
		if err := validateListOptions(options); err != nil {
			return err
		}
		ctx := context.Background()

		resources, hasState := listGatewaysFromState(ctx)
		if listStateOnly {
			if listLive {
				return fmt.Errorf("--state and --live cannot be used together")
			}
			if !hasState {
				return fmt.Errorf("no state context is active; configure one or use live AWS without --state")
			}
			rows := make([]map[string]any, 0, len(resources))
			for _, r := range resources {
				var trackedAt any
				if !r.CreatedAt.IsZero() {
					trackedAt = r.CreatedAt
				}
				rows = append(rows, map[string]any{
					"name": r.Name, "status": "-", "createdAt": "-", "region": r.Region,
					"createdBy": r.CreatedBy, "trackedAt": trackedAt, "arn": r.ARN,
				})
			}
			if err := aws.RenderList(rows, aws.GatewayListColumns(listWide), options); err != nil {
				return err
			}
			if !strings.EqualFold(listOutput, "json") {
				fmt.Println("\n(source: remote state; live AWS fields are unavailable)")
			}
			return nil
		}

		client, err := aws.NewECSClient(ctx, region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		summaries, err := client.ListGateways(ctx)
		if err != nil {
			return err
		}

		metaMap := make(map[string]state.Resource, len(resources))
		for _, r := range resources {
			metaMap[r.ARN] = r
			metaMap[r.Name] = r
		}

		rows := make([]map[string]any, 0, len(summaries))
		for _, s := range summaries {
			name := awssdk.ToString(s.Name)
			arn := awssdk.ToString(s.GatewayArn)
			var createdAt any
			if s.CreatedAt != nil {
				createdAt = s.CreatedAt.UTC()
			}
			trackedBy := "-"
			var trackedAt any = "-"
			if meta, ok := metaMap[arn]; ok {
				trackedBy = meta.CreatedBy
				if !meta.CreatedAt.IsZero() {
					trackedAt = meta.CreatedAt.UTC()
				}
			} else if meta, ok := metaMap[name]; ok {
				trackedBy = meta.CreatedBy
				if !meta.CreatedAt.IsZero() {
					trackedAt = meta.CreatedAt.UTC()
				}
			}

			rows = append(rows, map[string]any{
				"name":      name,
				"status":    string(s.Status),
				"createdAt": createdAt,
				"region":    client.Region(),
				"createdBy": trackedBy,
				"trackedAt": trackedAt,
				"arn":       arn,
			})
		}

		if err := aws.RenderList(rows, aws.GatewayListColumns(listWide), options); err != nil {
			return err
		}
		if !strings.EqualFold(listOutput, "json") {
			fmt.Println("\n(source: live AWS; tracking metadata from remote state where available)")
		}
		return nil
	},
}

// associateGatewayCmd links a gateway to a cluster.
var associateGatewayCmd = &cobra.Command{
	Use:     "associate",
	Short:   "Associate an ECS Gateway with a cluster",
	Example: `  ecsctl gateway associate --gateway my-gateway --cluster prod-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if assocGateway == "" {
			return fmt.Errorf("--gateway is required")
		}
		if assocCluster == "" {
			return fmt.Errorf("--cluster is required")
		}
		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return err
		}
		fmt.Printf("Associating gateway %q with cluster %q...\n", assocGateway, assocCluster)
		if err := client.AssociateGatewayWithCluster(context.Background(), assocGateway, assocCluster); err != nil {
			return err
		}
		fmt.Printf("✓ Associated gateway %q with cluster %q.\n", assocGateway, assocCluster)
		return nil
	},
}

// disassociateGatewayCmd removes a cluster link from a gateway.
var disassociateGatewayCmd = &cobra.Command{
	Use:     "disassociate",
	Short:   "Disassociate an ECS Gateway from a cluster",
	Example: `  ecsctl gateway disassociate --gateway my-gateway --cluster prod-cluster`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if assocGateway == "" {
			return fmt.Errorf("--gateway is required")
		}
		if assocCluster == "" {
			return fmt.Errorf("--cluster is required")
		}
		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return err
		}
		fmt.Printf("Disassociating gateway %q from cluster %q...\n", assocGateway, assocCluster)
		if err := client.DisassociateGatewayFromCluster(context.Background(), assocGateway, assocCluster); err != nil {
			return err
		}
		fmt.Printf("✓ Disassociated gateway %q from cluster %q.\n", assocGateway, assocCluster)
		return nil
	},
}

func listGatewaysFromState(ctx context.Context) ([]state.Resource, bool) {
	localCfg, err := localconfig.Load()
	if err != nil || (localCfg.CurrentContext == "" && stateContext == "") {
		return nil, false
	}
	_, activeCtx, err := localCfg.GetActiveContext(stateContext)
	if err != nil || activeCtx.Bucket == "" {
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
	return st.FindGateways(), true
}

func init() {
	// Flags for create gateway
	createGatewayCmd.Flags().StringVarP(&gatewayFile, "file", "f", "", "Path to gateway config YAML")
	createGatewayCmd.Flags().StringVar(&gatewayName, "name", "", "Gateway name")
	createGatewayCmd.Flags().StringVar(&gatewayInfraRole, "infra-role-arn", "", "IAM infrastructure role ARN for gateway management (auto-created if omitted)")
	createGatewayCmd.Flags().StringVar(&gatewayScheme, "scheme", "internet-facing", "Load balancer scheme: internet-facing or internal")
	createGatewayCmd.Flags().StringSliceVar(&gatewaySubnets, "subnets", nil, "Subnet IDs for managed load balancers (comma-separated)")
	createGatewayCmd.Flags().StringSliceVar(&gatewaySecurityGroups, "security-groups", nil, "Inbound security group IDs (comma-separated)")
	createGatewayCmd.Flags().Int32Var(&gatewayMaxALBs, "max-albs", 10, "Maximum number of Application Load Balancers the gateway manages")
	createGatewayCmd.Flags().StringVar(&gatewayWebACLArn, "web-acl-arn", "", "WAF Web ACL ARN to associate with the gateway")
	createGatewayCmd.Flags().BoolVar(&gatewayDeletionProtection, "deletion-protection", true, "Enable deletion protection on the gateway")
	createGatewayCmd.Flags().StringSliceVar(&gatewayClusters, "clusters", nil, "ECS clusters to associate with the gateway (comma-separated)")
	createGatewayCmd.Flags().StringArrayVar(&gatewayTags, "tags", nil, "Tags as key=value pairs (repeatable)")

	// Flags for describe gateway
	describeGatewayCmd.Flags().BoolVar(&gatewayShowResources, "resources", false, "Show managed load balancers and active service routes")

	// Flags for associate / disassociate
	associateGatewayCmd.Flags().StringVar(&assocGateway, "gateway", "", "Gateway name or ARN")
	associateGatewayCmd.Flags().StringVar(&assocCluster, "cluster", "", "Cluster name or ARN")
	_ = associateGatewayCmd.MarkFlagRequired("gateway")
	_ = associateGatewayCmd.MarkFlagRequired("cluster")

	disassociateGatewayCmd.Flags().StringVar(&assocGateway, "gateway", "", "Gateway name or ARN")
	disassociateGatewayCmd.Flags().StringVar(&assocCluster, "cluster", "", "Cluster name or ARN")
	_ = disassociateGatewayCmd.MarkFlagRequired("gateway")
	_ = disassociateGatewayCmd.MarkFlagRequired("cluster")

	// Register subcommands under create, describe, delete, list
	createCmd.AddCommand(createGatewayCmd)
	describeCmd.AddCommand(describeGatewayCmd)
	deleteCmd.AddCommand(deleteGatewayCmd)
	listCmd.AddCommand(listGatewaysCmd)

	// Create dedicated subcommands for the 'gateway' parent command
	gwCreateSubCmd := *createGatewayCmd
	gwCreateSubCmd.Use = "create"
	gwDescribeSubCmd := *describeGatewayCmd
	gwDescribeSubCmd.Use = "describe [name-or-arn]"
	gwDeleteSubCmd := *deleteGatewayCmd
	gwDeleteSubCmd.Use = "delete [name-or-arn]"
	gwListSubCmd := *listGatewaysCmd
	gwListSubCmd.Use = "list"

	gatewayCmd.AddCommand(&gwCreateSubCmd)
	gatewayCmd.AddCommand(&gwDescribeSubCmd)
	gatewayCmd.AddCommand(&gwDeleteSubCmd)
	gatewayCmd.AddCommand(&gwListSubCmd)
	gatewayCmd.AddCommand(associateGatewayCmd)
	gatewayCmd.AddCommand(disassociateGatewayCmd)
	rootCmd.AddCommand(gatewayCmd)
}
