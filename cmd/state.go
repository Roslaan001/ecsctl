package cmd

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	ecsaws "github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/localconfig"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
)

// stateCmd is the parent for all "state" subcommands.
var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage ecsctl remote state",
}

// ---------- state init ----------

var (
	stateInitContext string
	stateInitBucket  string
	stateInitRegion  string
	stateInitProfile string
	stateInitKey     string
	stateInitKmsKey  string
)

var stateInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise a remote state backend (S3 bucket) and save it as a named context",
	Example: `  ecsctl state init --context prod --bucket my-ecsctl-prod-state --region eu-west-2
  ecsctl state init --context staging --bucket my-ecsctl-staging-state --region eu-west-1`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Load AWS config
		var opts []func(*awscfg.LoadOptions) error
		opts = append(opts, awscfg.WithRegion(stateInitRegion))
		if stateInitProfile != "" {
			opts = append(opts, awscfg.WithSharedConfigProfile(stateInitProfile))
		}
		cfg, err := awscfg.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return fmt.Errorf("loading AWS config: %w", err)
		}
		s3Client := s3.NewFromConfig(cfg)

		// Create bucket if it doesn't already exist
		fmt.Printf("Checking bucket s3://%s...\n", stateInitBucket)
		_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(stateInitBucket),
		})
		if err != nil {
			// Bucket doesn't exist — create it
			fmt.Printf("Creating bucket s3://%s in %s...\n", stateInitBucket, stateInitRegion)
			createInput := &s3.CreateBucketInput{
				Bucket: aws.String(stateInitBucket),
			}
			// us-east-1 must NOT specify a LocationConstraint
			if stateInitRegion != "us-east-1" {
				createInput.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
					LocationConstraint: s3types.BucketLocationConstraint(stateInitRegion),
				}
			}
			if _, err := s3Client.CreateBucket(ctx, createInput); err != nil {
				return fmt.Errorf("creating bucket: %w", err)
			}
			fmt.Printf("✓ Bucket s3://%s created.\n", stateInitBucket)
		} else {
			fmt.Printf("✓ Bucket s3://%s already exists.\n", stateInitBucket)
		}

		// Enable versioning so state history is preserved
		_, err = s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
			Bucket: aws.String(stateInitBucket),
			VersioningConfiguration: &s3types.VersioningConfiguration{
				Status: s3types.BucketVersioningStatusEnabled,
			},
		})
		if err != nil {
			return fmt.Errorf("enabling versioning: %w", err)
		}
		fmt.Println("✓ Versioning enabled.")

		// Default key prefix to the context name if not specified
		if stateInitKey == "" {
			stateInitKey = stateInitContext
		}

		// Save context to ~/.ecsctl/config.yaml
		localCfg, err := localconfig.Load()
		if err != nil {
			return fmt.Errorf("loading local config: %w", err)
		}

		localCfg.AddContext(stateInitContext, localconfig.Context{
			Bucket:   stateInitBucket,
			Region:   stateInitRegion,
			Key:      stateInitKey,
			Profile:  stateInitProfile,
			KmsKeyID: stateInitKmsKey,
		}, true)

		if err := localconfig.Save(localCfg); err != nil {
			return fmt.Errorf("saving local config: %w", err)
		}

		fmt.Printf("✓ Context %q saved to ~/.ecsctl/config.yaml\n", stateInitContext)
		fmt.Printf("✓ Active context set to %q\n", stateInitContext)
		return nil
	},
}

// ---------- state use-context ----------

var stateUseContextCmd = &cobra.Command{
	Use:     "use-context [name]",
	Short:   "Switch the active context",
	Args:    cobra.ExactArgs(1),
	Example: `  ecsctl state use-context staging`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		localCfg, err := localconfig.Load()
		if err != nil {
			return fmt.Errorf("loading local config: %w", err)
		}
		if _, ok := localCfg.Contexts[name]; !ok {
			return fmt.Errorf("context %q not found — run 'ecsctl state init --context %s' first", name, name)
		}

		localCfg.CurrentContext = name
		if err := localconfig.Save(localCfg); err != nil {
			return fmt.Errorf("saving local config: %w", err)
		}

		fmt.Printf("✓ Switched to context %q\n", name)
		return nil
	},
}

// ---------- state list-contexts ----------

var stateListContextsCmd = &cobra.Command{
	Use:     "list-contexts",
	Short:   "List all configured contexts",
	Example: `  ecsctl state list-contexts`,
	RunE: func(cmd *cobra.Command, args []string) error {
		localCfg, err := localconfig.Load()
		if err != nil {
			return fmt.Errorf("loading local config: %w", err)
		}
		if len(localCfg.Contexts) == 0 {
			fmt.Println("No contexts configured. Run 'ecsctl state init' to get started.")
			return nil
		}

		fmt.Printf("%-4s %-20s %-35s %-20s %s\n", "", "NAME", "BUCKET", "KEY PREFIX", "REGION")
		fmt.Println("--------------------------------------------------------------------------------")
		for name, ctx := range localCfg.Contexts {
			active := " "
			if name == localCfg.CurrentContext {
				active = "*"
			}
			fmt.Printf("%-4s %-20s %-35s %-20s %s\n", active, name, ctx.Bucket, ctx.Key, ctx.Region)
		}
		return nil
	},
}

// ---------- state import ----------

var (
	stateImportContext string
	stateImportCluster string
)

var stateImportCmd = &cobra.Command{
	Use:   "import [resource-type] [name]",
	Short: "Import an existing AWS resource into ecsctl state",
	Args:  cobra.ExactArgs(2),
	Example: `  ecsctl state import cluster my-cluster --region eu-west-2
  ecsctl state import service my-service --cluster my-cluster --region eu-west-2
  ecsctl state import express my-api --cluster my-cluster --region eu-west-2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceType := args[0]
		resourceName := args[1]
		ctx := context.Background()

		// Resolve context
		localCfg, err := localconfig.Load()
		if err != nil {
			return fmt.Errorf("loading local config: %w", err)
		}
		_, activeCtx, err := localCfg.GetActiveContext(stateImportContext)
		if err != nil {
			return err
		}

		resolvedRegion := region
		if resolvedRegion == "" {
			resolvedRegion = activeCtx.Region
		}
		resolvedProfile := profile
		if resolvedProfile == "" {
			resolvedProfile = activeCtx.Profile
		}

		// Describe the resource from AWS to confirm it exists
		ecsClient, err := ecsaws.NewECSClient(ctx, resolvedRegion, resolvedProfile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		var res state.Resource

		switch resourceType {
		case "cluster":
			r, err := ecsClient.DescribeClusterResource(ctx, resourceName)
			if err != nil {
				return err
			}
			res = *r

		case "service":
			if stateImportCluster == "" {
				return fmt.Errorf("--cluster is required when importing a service")
			}
			r, err := ecsClient.DescribeServiceResource(ctx, stateImportCluster, resourceName)
			if err != nil {
				return err
			}
			res = *r

		case "express", "express-service":
			services, err := ecsClient.ListExpressServices(ctx, stateImportCluster)
			if err != nil {
				return err
			}
			var arn string
			for _, service := range services {
				if aws.ToString(service.ServiceName) == resourceName || aws.ToString(service.ServiceArn) == resourceName {
					arn = aws.ToString(service.ServiceArn)
					break
				}
			}
			if arn == "" {
				return fmt.Errorf("Express service %q not found", resourceName)
			}
			r, err := ecsClient.DescribeExpressServiceResource(ctx, arn)
			if err != nil {
				return err
			}
			res = *r

		default:
			return fmt.Errorf("unknown resource type %q — supported: cluster, service, express", resourceType)
		}

		// Load state, add resource, save
		backend, err := state.NewBackend(ctx, activeCtx.Bucket, activeCtx.Key, resolvedRegion, resolvedProfile, activeCtx.KmsKeyID)
		if err != nil {
			return err
		}

		if err := backend.Lock(ctx); err != nil {
			return err
		}
		defer backend.Unlock(ctx) //nolint:errcheck

		st, err := backend.Load(ctx)
		if err != nil {
			return err
		}

		st.AddResource(res)

		if err := backend.Save(ctx, st); err != nil {
			return err
		}

		fmt.Printf("✓ Imported %s %q into state (context: %s)\n", resourceType, resourceName, localCfg.CurrentContext)
		return nil
	},
}

// ---------- state show ----------

var stateShowCmd = &cobra.Command{
	Use:     "show",
	Short:   "Print the contents of the current state",
	Example: `  ecsctl state show`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		localCfg, err := localconfig.Load()
		if err != nil {
			return fmt.Errorf("loading local config: %w", err)
		}
		ctxName, activeCtx, err := localCfg.GetActiveContext(stateContext)
		if err != nil {
			return err
		}

		backend, err := state.NewBackend(ctx, activeCtx.Bucket, activeCtx.Key, activeCtx.Region, activeCtx.Profile, activeCtx.KmsKeyID)
		if err != nil {
			return err
		}

		st, err := backend.Load(ctx)
		if err != nil {
			return err
		}

		fmt.Printf("Context:  %s\n", ctxName)
		fmt.Printf("Bucket:   s3://%s/%s/state.json\n", activeCtx.Bucket, activeCtx.Key)
		fmt.Printf("Updated:  %s\n", st.UpdatedAt.Format("2006-01-02 15:04:05 UTC"))
		fmt.Printf("Resources: %d\n", len(st.Resources))
		fmt.Println()

		clusters := st.FindClusters()
		if len(clusters) > 0 {
			fmt.Printf("CLUSTERS (%d)\n", len(clusters))
			fmt.Printf("  %-30s %-12s %-15s %s\n", "NAME", "REGION", "CREATED BY", "CREATED AT")
			fmt.Println("  " + "─────────────────────────────────────────────────────────────")
			for _, r := range clusters {
				createdAt := ""
				if !r.CreatedAt.IsZero() {
					createdAt = r.CreatedAt.Format("2006-01-02 15:04")
				}
				fmt.Printf("  %-30s %-12s %-15s %s\n", r.Name, r.Region, r.CreatedBy, createdAt)
			}
			fmt.Println()
		}

		services := st.FindServices("")
		if len(services) > 0 {
			fmt.Printf("SERVICES (%d)\n", len(services))
			fmt.Printf("  %-30s %-20s %-12s %-15s %s\n", "NAME", "CLUSTER", "REGION", "CREATED BY", "CREATED AT")
			fmt.Println("  " + "─────────────────────────────────────────────────────────────────────────────────")
			for _, r := range services {
				createdAt := ""
				if !r.CreatedAt.IsZero() {
					createdAt = r.CreatedAt.Format("2006-01-02 15:04")
				}
				fmt.Printf("  %-30s %-20s %-12s %-15s %s\n", r.Name, r.Cluster, r.Region, r.CreatedBy, createdAt)
			}
		}

		expressServices := st.FindExpressServices("")
		if len(expressServices) > 0 {
			fmt.Printf("EXPRESS SERVICES (%d)\n", len(expressServices))
			fmt.Printf("  %-30s %-20s %-12s %-14s %s\n", "NAME", "CLUSTER", "REGION", "CONFIG", "ARN")
			for _, resource := range expressServices {
				configStatus := "not captured"
				if resource.Configuration != "" {
					configStatus = "captured"
				}
				fmt.Printf("  %-30s %-20s %-12s %-14s %s\n", resource.Name, resource.Cluster, resource.Region, configStatus, resource.ARN)
			}
		}

		if len(st.Resources) == 0 {
			fmt.Println("No resources tracked. Use 'ecsctl state import' to add existing resources.")
		}

		return nil
	},
}

var (
	stateConfigCluster string
)

var stateConfigCmd = &cobra.Command{
	Use:   "config [cluster|service|express] [name]",
	Short: "Print the saved configuration for a tracked resource",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		localCfg, err := localconfig.Load()
		if err != nil {
			return fmt.Errorf("loading local config: %w", err)
		}
		_, activeCtx, err := localCfg.GetActiveContext(stateContext)
		if err != nil {
			return err
		}
		backend, err := state.NewBackend(context.Background(), activeCtx.Bucket, activeCtx.Key, activeCtx.Region, activeCtx.Profile, activeCtx.KmsKeyID)
		if err != nil {
			return err
		}
		st, err := backend.Load(context.Background())
		if err != nil {
			return err
		}
		resourceType := state.ResourceType(args[0])
		if resourceType == "express" {
			resourceType = state.ResourceTypeExpressService
		}
		for _, resource := range st.Resources {
			if resource.Type != resourceType || resource.Name != args[1] || (stateConfigCluster != "" && resource.Cluster != stateConfigCluster) {
				continue
			}
			if resource.Configuration == "" {
				return fmt.Errorf("configuration was not captured for imported %s %q", args[0], args[1])
			}
			fmt.Print(resource.Configuration)
			return nil
		}
		return fmt.Errorf("%s %q was not found in the active state", args[0], args[1])
	},
}

func init() {
	// state init flags
	stateInitCmd.Flags().StringVar(&stateInitContext, "context", "default", "Context name to create")
	stateInitCmd.Flags().StringVar(&stateInitBucket, "bucket", "", "S3 bucket name for remote state (required)")
	stateInitCmd.Flags().StringVar(&stateInitRegion, "region", "", "AWS region for the S3 bucket (required)")
	stateInitCmd.Flags().StringVar(&stateInitKey, "key", "", "S3 key prefix for state files (defaults to context name, e.g. 'prod' → prod/state.json)")
	stateInitCmd.Flags().StringVar(&stateInitProfile, "profile", "", "AWS profile to use")
	stateInitCmd.Flags().StringVar(&stateInitKmsKey, "kms-key-id", "", "KMS Key ID or ARN for state file encryption (optional)")
	_ = stateInitCmd.MarkFlagRequired("bucket")
	_ = stateInitCmd.MarkFlagRequired("region")

	// state import flags
	stateImportCmd.Flags().StringVar(&stateImportContext, "context", "", "Context to import into (defaults to current context)")
	stateImportCmd.Flags().StringVar(&stateImportCluster, "cluster", "", "Cluster name (required for service imports)")

	stateCmd.AddCommand(stateInitCmd)
	stateCmd.AddCommand(stateUseContextCmd)
	stateCmd.AddCommand(stateListContextsCmd)
	stateCmd.AddCommand(stateImportCmd)
	stateCmd.AddCommand(stateShowCmd)
	stateConfigCmd.Flags().StringVar(&stateConfigCluster, "cluster", "", "Cluster name when selecting a service")
	stateCmd.AddCommand(stateConfigCmd)
}
