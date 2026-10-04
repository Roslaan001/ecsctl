package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	region       string
	profile      string
	stateContext string // named state context, e.g. "prod", "staging"
)

var rootCmd = &cobra.Command{
	Use:   "ecsctl",
	Short: "A CLI tool for managing Amazon ECS resources",
	Long: `ecsctl is a command-line tool for creating and managing Amazon ECS
clusters, services, and tasks — inspired by eksctl.

Examples:
  ecsctl create cluster -f cluster.yaml
  ecsctl create service -f service.yaml
  ecsctl deploy my-service --image nginx:1.25
  ecsctl rollback my-service --cluster my-cluster
  ecsctl logs my-service --tail 100
  ecsctl exec my-service --container web
  ecsctl scale my-service --desired 3
  ecsctl delete service my-service
  ecsctl delete cluster my-cluster`,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringVar(&region, "region", "", "AWS region (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "AWS profile to use")
	rootCmd.PersistentFlags().StringVar(&stateContext, "context", "", "State context to use (overrides current context in ~/.ecsctl/config.yaml)")

	// Register subcommand groups
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(scaleCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(describeCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(completionCmd)
}
