package cmd

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecsaws "github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/config"
	"github.com/spf13/cobra"
)

var expressFile, expressARN string
var expressWait bool

var expressCmd = &cobra.Command{Use: "express", Short: "Manage ECS Express Mode services"}

var expressCreateCmd = &cobra.Command{
	Use: "create", Short: "Create an ECS Express Mode service",
	Example: "ecsctl express create -f express.yaml --wait",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadExpressServiceConfig(expressFile)
		if err != nil {
			return err
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
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadExpressServiceConfig(expressFile)
		if err != nil {
			return err
		}
		client, err := ecsaws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return err
		}
		if err := client.UpdateExpressService(context.Background(), expressARN, cfg); err != nil {
			return err
		}
		fmt.Printf("Updated Express service %s\n", expressARN)
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
	fmt.Printf("Name: %s\nARN: %s\nCluster: %s\nStatus: %s\nReason: %s\n", awssdk.ToString(service.ServiceName), awssdk.ToString(service.ServiceArn), awssdk.ToString(service.Cluster), status, reason)
	printExpressIngress(service)
	return nil
}}

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

var expressDeleteCmd = &cobra.Command{Use: "delete", Short: "Delete an ECS Express Mode service", RunE: func(cmd *cobra.Command, args []string) error {
	client, err := ecsaws.NewECSClient(context.Background(), region, profile)
	if err != nil {
		return err
	}
	if err := client.DeleteExpressService(context.Background(), expressARN); err != nil {
		return err
	}
	fmt.Printf("Deletion started for Express service %s\n", expressARN)
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
	for _, command := range []*cobra.Command{expressDescribeCmd, expressDeleteCmd} {
		command.Flags().StringVar(&expressARN, "service-arn", "", "Express service ARN (required)")
		_ = command.MarkFlagRequired("service-arn")
	}
	expressCmd.AddCommand(expressCreateCmd, expressUpdateCmd, expressDescribeCmd, expressDeleteCmd)
	rootCmd.AddCommand(expressCmd)
}
