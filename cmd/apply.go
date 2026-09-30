package cmd

import (
	"context"
	"fmt"
	"os"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/config"
	"github.com/spf13/cobra"
)

var (
	applyFile   string
	applyWait   bool
	applyDryRun bool
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a config file — create resources if absent, update if changed",
	Example: `  ecsctl apply -f cluster.yaml
  ecsctl apply -f service.yaml --wait
  ecsctl apply -f service.yaml --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Detect resource type from the file
		resourceType, err := config.DetectResourceType(applyFile)
		if err != nil {
			return fmt.Errorf("reading config: %w", err)
		}

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		switch resourceType {
		case "cluster":
			return applyCluster(client, applyFile)
		case "service":
			return applyService(client, applyFile)
		case "express-service":
			return applyExpressService(client, applyFile)
		default:
			return fmt.Errorf("unknown resource kind %q in %s", resourceType, applyFile)
		}
	},
}

func applyCluster(client *aws.Client, file string) error {
	cfg, err := config.LoadClusterConfig(file)
	if err != nil {
		return err
	}

	resolvedRegion := region
	if resolvedRegion == "" {
		resolvedRegion = cfg.Region
	}

	exists, err := client.ClusterExists(context.Background(), cfg.Name)
	if err != nil {
		return err
	}

	if exists {
		changed, err := client.ReconcileCluster(context.Background(), cfg, applyDryRun)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Printf("  Cluster %q is up to date.\n", cfg.Name)
		}
		if !applyDryRun {
			if err := writeClusterState(cfg, resolvedRegion); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: state not updated: %v\n", err)
			}
		}
		return nil
	}

	if applyDryRun {
		fmt.Printf("[dry-run] + Would create cluster %q in %s.\n", cfg.Name, resolvedRegion)
		return nil
	}

	fmt.Printf("+ Creating cluster %q in %s...\n", cfg.Name, resolvedRegion)
	if err := client.CreateCluster(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	fmt.Printf("✓ Cluster %q created.\n", cfg.Name)

	if err := writeClusterState(cfg, resolvedRegion); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: state not updated: %v\n", err)
	}
	return nil
}

func applyService(client *aws.Client, file string) error {
	cfg, err := config.LoadServiceConfig(file)
	if err != nil {
		return err
	}

	exists, err := client.ServiceExists(context.Background(), cfg.Cluster, cfg.Name)
	if err != nil {
		return err
	}

	if exists {
		// Service exists — check if desired count changed and update if needed
		fmt.Printf("~ Service %q already exists — checking for drift...\n", cfg.Name)
		changed, err := client.ReconcileService(context.Background(), cfg, applyDryRun)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Printf("  No changes required.\n")
		}
		if !applyDryRun {
			if err := writeServiceState(cfg, region); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: state not updated: %v\n", err)
			}
		} else {
			fmt.Printf("[dry-run] Would ensure service %q in cluster %q is tracked in state.\n", cfg.Name, cfg.Cluster)
		}
		return nil
	}

	if applyDryRun {
		fmt.Printf("[dry-run] + Would create service %q in cluster %q.\n", cfg.Name, cfg.Cluster)
		return nil
	}

	fmt.Printf("+ Creating service %q in cluster %q...\n", cfg.Name, cfg.Cluster)
	if err := client.CreateService(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	fmt.Printf("✓ Service %q created.\n", cfg.Name)

	if applyWait {
		fmt.Println("Waiting for service to reach steady state...")
		if err := client.WaitForServiceStable(context.Background(), cfg.Cluster, cfg.Name); err != nil {
			return err
		}
		fmt.Printf("✓ Service %q is stable.\n", cfg.Name)
	}

	if err := writeServiceState(cfg, region); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: state not updated: %v\n", err)
	}
	return nil
}

func applyExpressService(client *aws.Client, file string) error {
	cfg, err := config.LoadExpressServiceConfig(file)
	if err != nil {
		return err
	}
	services, err := client.ListExpressServices(context.Background(), cfg.Cluster)
	if err != nil {
		return err
	}
	var existingARN string
	for _, service := range services {
		if awssdk.ToString(service.ServiceName) == cfg.ServiceName {
			existingARN = awssdk.ToString(service.ServiceArn)
			break
		}
	}
	if existingARN != "" {
		changed, err := client.ReconcileExpressService(context.Background(), existingARN, cfg, applyDryRun)
		if err != nil {
			return err
		}
		if !changed {
			fmt.Printf("  Express service %q is up to date.\n", cfg.ServiceName)
		}
		if applyDryRun {
			return nil
		}
		if err := writeExpressState(cfg, existingARN, region); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Express service updated but state not updated: %v\n", err)
		}
		if applyWait {
			if err := client.WaitForExpressService(context.Background(), existingARN); err != nil {
				return err
			}
			service, err := client.DescribeExpressService(context.Background(), existingARN)
			if err != nil {
				return err
			}
			printExpressIngress(service)
		}
		return nil
	}
	if applyDryRun {
		fmt.Printf("[dry-run] + Would create Express service %q.\n", cfg.ServiceName)
		return nil
	}
	arn, err := client.CreateExpressService(context.Background(), cfg)
	if err != nil {
		return err
	}
	if err := writeExpressState(cfg, arn, region); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Express service created but state not updated: %v\n", err)
	}
	fmt.Printf("✓ Express service %q created (%s).\n", cfg.ServiceName, arn)
	if applyWait {
		if err := client.WaitForExpressService(context.Background(), arn); err != nil {
			return err
		}
		service, err := client.DescribeExpressService(context.Background(), arn)
		if err != nil {
			return err
		}
		printExpressIngress(service)
	}
	return nil
}

func init() {
	applyCmd.Flags().StringVarP(&applyFile, "file", "f", "", "Path to config YAML (required)")
	applyCmd.Flags().BoolVar(&applyWait, "wait", false, "Wait for the resource to be stable after creation")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Print the changes that would be made without applying them")
	_ = applyCmd.MarkFlagRequired("file")
}
