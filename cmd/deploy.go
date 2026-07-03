package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var (
	deployCluster   string
	deployImage     string
	deployContainer string
	deployWait      bool
)

var deployCmd = &cobra.Command{
	Use:   "deploy [service]",
	Short: "Deploy a new image to an ECS service",
	Args:  cobra.ExactArgs(1),
	Example: `  ecsctl deploy my-service --image nginx:1.25 --cluster my-cluster
  ecsctl deploy my-service --image nginx:1.25 --cluster my-cluster --container web
  ecsctl deploy my-service --image nginx:1.25 --cluster my-cluster --wait`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		fmt.Printf("Deploying image %q to service %q in cluster %q...\n", deployImage, serviceName, deployCluster)
		newTaskDef, err := client.DeployService(context.Background(), deployCluster, serviceName, deployContainer, deployImage)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Deployment triggered for service %q (task def: %s).\n", serviceName, newTaskDef)

		if deployWait {
			fmt.Println("Waiting for deployment to complete...")
			if err := client.WaitForDeployment(context.Background(), deployCluster, serviceName, newTaskDef); err != nil {
				fmt.Fprintf(os.Stderr, "Deployment failed: %v\n", err)
				return err
			}
			fmt.Printf("✓ Deployment complete. Service %q is stable.\n", serviceName)
		} else {
			fmt.Println("Tip: use --wait to poll until the deployment is complete.")
		}

		return nil
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployCluster, "cluster", "", "ECS cluster name (required)")
	deployCmd.Flags().StringVar(&deployImage, "image", "", "Docker image to deploy, e.g. nginx:1.25 (required)")
	deployCmd.Flags().StringVar(&deployContainer, "container", "", "Container name to update (defaults to first container in task def)")
	deployCmd.Flags().BoolVar(&deployWait, "wait", false, "Wait for the deployment to complete before exiting")
	_ = deployCmd.MarkFlagRequired("cluster")
	_ = deployCmd.MarkFlagRequired("image")
}
