package cmd

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecsaws "github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/config"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
)

var expressFile, expressARN string
var expressListCluster string
var expressWait bool

var expressCmd = &cobra.Command{Use: "express", Short: "Manage ECS Express Mode services"}

var expressCreateCmd = &cobra.Command{
	Use: "create", Short: "Create an ECS Express Mode service",
	Example: "ecsctl express create -f express.yaml --wait",
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		cfg, err := config.LoadExpressServiceConfig(expressFile)
		if err != nil {
			return err
		}
		session, err := beginStateSession(context.Background())
		if err != nil {
			return fmt.Errorf("locking remote state: %w", err)
		}
		if session != nil {
			defer closeStateSessionOnReturn(session, &runErr, "releasing remote state lock")
		}
		client, err := ecsaws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return err
		}
		arn, err := client.CreateExpressService(context.Background(), cfg)
		if err != nil {
			return err
		}
		fmt.Printf("Created Express service %q (%s)\n", cfg.ServiceName, arn)
		if session != nil {
			if err := recordExpressState(session, client, cfg, arn, region); err != nil {
				return fmt.Errorf("express service created but remote state was not updated: %w", err)
			}
		}
		if expressWait {
			fmt.Println("Waiting for service to become active...")
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
	},
}

var expressUpdateCmd = &cobra.Command{
	Use: "update", Short: "Update an ECS Express Mode service",
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		cfg, err := config.LoadExpressServiceConfig(expressFile)
		if err != nil {
			return err
		}
		session, err := beginStateSession(context.Background())
		if err != nil {
			return fmt.Errorf("locking remote state: %w", err)
		}
		if session != nil {
			defer closeStateSessionOnReturn(session, &runErr, "releasing remote state lock")
		}
		client, err := ecsaws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return err
		}
		if err := client.UpdateExpressService(context.Background(), expressARN, cfg); err != nil {
			return err
		}
		fmt.Printf("Updated Express service %s\n", expressARN)
		if session != nil {
			if err := recordExpressState(session, client, cfg, expressARN, region); err != nil {
				return fmt.Errorf("express service updated but remote state was not updated: %w", err)
			}
		}
		if expressWait {
			if err := client.WaitForExpressService(context.Background(), expressARN); err != nil {
				return err
			}
			service, err := client.DescribeExpressService(context.Background(), expressARN)
			if err != nil {
				return err
			}
			printExpressIngress(service)
		}
		return nil
	},
}

var expressDescribeCmd = &cobra.Command{Use: "describe", Short: "Show an ECS Express Mode service", RunE: func(cmd *cobra.Command, args []string) error {
	client, err := ecsaws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	service, err := client.DescribeExpressService(context.Background(), expressARN)
	if err != nil {
		return err
	}
	if service == nil {
		return fmt.Errorf("ECS returned no service for %s", expressARN)
	}
	status, reason := "", ""
	if service.Status != nil {
		status = string(service.Status.StatusCode)
		reason = awssdk.ToString(service.Status.StatusReason)
	}
	printExpressDetails(service, status, reason)
	return nil
}}

var expressListCmd = &cobra.Command{Use: "list", Short: "List ECS Express Mode services", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
	client, err := ecsaws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	services, err := client.ListExpressServices(context.Background(), expressListCluster)
	if err != nil {
		return err
	}
	if len(services) == 0 {
		fmt.Println("No Express services found.")
		return nil
	}
	fmt.Printf("%-28s %-18s %-14s %s\n", "NAME", "CLUSTER", "STATUS", "ARN")
	for _, service := range services {
		status := ""
		if service.Status != nil {
			status = string(service.Status.StatusCode)
		}
		fmt.Printf("%-28s %-18s %-14s %s\n", awssdk.ToString(service.ServiceName), awssdk.ToString(service.Cluster), status, awssdk.ToString(service.ServiceArn))
	}
	return nil
}}

func printExpressDetails(service *types.ECSExpressGatewayService, status, reason string) {
	fmt.Printf("Name: %s\nARN: %s\nCluster: %s\nStatus: %s\nReason: %s\nInfrastructure role: %s\nCurrent deployment: %s\n",
		awssdk.ToString(service.ServiceName), awssdk.ToString(service.ServiceArn), awssdk.ToString(service.Cluster), status, reason,
		awssdk.ToString(service.InfrastructureRoleArn), awssdk.ToString(service.CurrentDeployment))
	if service.UpdatedAt != nil {
		fmt.Printf("Updated: %s\n", service.UpdatedAt.UTC().Format("2006-01-02 15:04:05 UTC"))
	}
	for index, revision := range service.ActiveConfigurations {
		fmt.Printf("\nActive configuration %d\n", index+1)
		fmt.Printf("  Revision: %s\n  Task definition: %s\n  CPU: %s\n  Memory: %s\n  Architecture: %s\n  Health check: %s\n",
			awssdk.ToString(revision.ServiceRevisionArn), awssdk.ToString(revision.TaskDefinitionArn), awssdk.ToString(revision.Cpu), awssdk.ToString(revision.Memory), revision.CpuArchitecture, awssdk.ToString(revision.HealthCheckPath))
		if revision.PrimaryContainer != nil {
			container := revision.PrimaryContainer
			fmt.Printf("  Image: %s\n  Container port: %d\n", awssdk.ToString(container.Image), awssdk.ToInt32(container.ContainerPort))
			if logs := container.AwsLogsConfiguration; logs != nil {
				fmt.Printf("  Log group: %s\n  Log stream prefix: %s\n", awssdk.ToString(logs.LogGroup), awssdk.ToString(logs.LogStreamPrefix))
			}
		}
		if scaling := revision.ScalingTarget; scaling != nil {
			fmt.Printf("  Scaling: %s target=%d min=%d max=%d\n", scaling.AutoScalingMetric, awssdk.ToInt32(scaling.AutoScalingTargetValue), awssdk.ToInt32(scaling.MinTaskCount), awssdk.ToInt32(scaling.MaxTaskCount))
		}
		for _, ingress := range revision.IngressPaths {
			fmt.Printf("  %s ingress: %s\n", ingress.AccessType, awssdk.ToString(ingress.Endpoint))
		}
	}
}

func printExpressIngress(service *types.ECSExpressGatewayService) {
	if service == nil {
		return
	}
	for _, revision := range service.ActiveConfigurations {
		for _, ingress := range revision.IngressPaths {
			fmt.Printf("%s: %s\n", ingress.AccessType, awssdk.ToString(ingress.Endpoint))
		}
	}
}

var expressDeleteCmd = &cobra.Command{Use: "delete", Short: "Delete an ECS Express Mode service", RunE: func(cmd *cobra.Command, args []string) (runErr error) {
	session, err := beginStateSession(context.Background())
	if err != nil {
		return fmt.Errorf("locking remote state: %w", err)
	}
	if session != nil {
		defer closeStateSessionOnReturn(session, &runErr, "releasing remote state lock")
	}
	client, err := ecsaws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	if err := client.DeleteExpressService(context.Background(), expressARN); err != nil {
		return err
	}
	fmt.Printf("Deletion started for Express service %s\n", expressARN)
	if session != nil {
		if err := session.Update(func(st *state.State) { st.RemoveResourceByARN(expressARN) }); err != nil {
			return fmt.Errorf("express service deletion started but remote state was not updated: %w", err)
		}
	}
	return nil
}}

func init() {
	expressCreateCmd.Flags().StringVarP(&expressFile, "file", "f", "", "Express service YAML configuration (required)")
	_ = expressCreateCmd.MarkFlagRequired("file")
	expressCreateCmd.Flags().BoolVar(&expressWait, "wait", true, "Wait for provisioning to complete")
	expressUpdateCmd.Flags().StringVarP(&expressFile, "file", "f", "", "Express service YAML configuration (required)")
	expressUpdateCmd.Flags().StringVar(&expressARN, "service-arn", "", "Express service ARN (required)")
	_ = expressUpdateCmd.MarkFlagRequired("file")
	_ = expressUpdateCmd.MarkFlagRequired("service-arn")
	expressUpdateCmd.Flags().BoolVar(&expressWait, "wait", true, "Wait for update to complete")
	expressListCmd.Flags().StringVar(&expressListCluster, "cluster", "", "ECS cluster name (defaults to the default cluster)")
	for _, command := range []*cobra.Command{expressDescribeCmd, expressDeleteCmd} {
		command.Flags().StringVar(&expressARN, "service-arn", "", "Express service ARN (required)")
		_ = command.MarkFlagRequired("service-arn")
	}
	expressCmd.AddCommand(expressCreateCmd, expressUpdateCmd, expressDescribeCmd, expressDeleteCmd, expressListCmd)
	rootCmd.AddCommand(expressCmd)
}
