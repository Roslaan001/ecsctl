package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var taskDefinitionStatus string

var listTaskDefinitionsCmd = &cobra.Command{
	Use:     "task-definitions",
	Aliases: []string{"task-definition", "task-defs"},
	Short:   "List task definition revisions",
	Example: "  ecsctl list task-definitions --status ACTIVE",
	RunE: func(cmd *cobra.Command, args []string) error {
		status := types.TaskDefinitionStatus(strings.ToUpper(taskDefinitionStatus))
		switch status {
		case types.TaskDefinitionStatusActive, types.TaskDefinitionStatusInactive, types.TaskDefinitionStatusDeleteInProgress:
		default:
			return fmt.Errorf("invalid task definition status %q (choose ACTIVE, INACTIVE, or DELETE_IN_PROGRESS)", taskDefinitionStatus)
		}

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		return client.ListTaskDefinitions(context.Background(), status, listWide)
	},
}

func init() {
	listTaskDefinitionsCmd.Flags().StringVar(&taskDefinitionStatus, "status", "ACTIVE", "Task definition status: ACTIVE, INACTIVE, or DELETE_IN_PROGRESS")
}
