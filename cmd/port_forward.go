package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/spf13/cobra"
)

var (
	portForwardCluster       string
	portForwardContainer     string
	portForwardTask          string
	portForwardLocalPort     int
	portForwardContainerPort int
)

var portForwardCmd = &cobra.Command{
	Use:   "port-forward [service] [localPort:containerPort]",
	Short: "Forward a local port to a container port in an ECS task",
	Long: `Forward a local port to a container port in an Amazon ECS task using AWS Systems Manager.

Requires the AWS Session Manager plugin to be installed locally, and ECS Exec to be enabled
on the target ECS service/task.`,
	Args: cobra.RangeArgs(0, 2),
	Example: `  # Forward local port 8080 to container port 80 on a service
  ecsctl port-forward my-service 8080:80 --cluster my-cluster

  # Forward local port 8080 to container port 8080
  ecsctl port-forward my-service 8080 --cluster my-cluster

  # Forward to a specific container in a multi-container task
  ecsctl port-forward my-service 8080:80 --cluster my-cluster --container web

  # Forward directly to a specific task
  ecsctl port-forward 8080:80 --cluster my-cluster --task abc123def456`,
	RunE: func(cmd *cobra.Command, args []string) error {
		service, task, localPort, containerPort, err := parsePortForwardArgs(
			args,
			portForwardTask,
			portForwardLocalPort,
			portForwardContainerPort,
		)
		if err != nil {
			return err
		}

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		opts := aws.PortForwardOptions{
			Cluster:       portForwardCluster,
			Service:       service,
			TaskID:        task,
			Container:     portForwardContainer,
			LocalPort:     localPort,
			ContainerPort: containerPort,
			Profile:       profile,
		}

		return client.PortForward(context.Background(), opts)
	},
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func parsePortSpec(spec string) (localPort, containerPort int, err error) {
	if strings.Contains(spec, ":") {
		parts := strings.Split(spec, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return 0, 0, fmt.Errorf("invalid port format %q; expected [localPort]:[containerPort] (e.g. 8080:80)", spec)
		}
		lp, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid local port %q: must be an integer", parts[0])
		}
		cp, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid container port %q: must be an integer", parts[1])
		}
		return lp, cp, nil
	}

	p, err := strconv.Atoi(spec)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid port %q: must be an integer", spec)
	}
	return p, p, nil
}

func parsePortForwardArgs(args []string, taskFlag string, localPortFlag, containerPortFlag int) (service, task string, localPort, containerPort int, err error) {
	task = taskFlag
	localPort = localPortFlag
	containerPort = containerPortFlag

	var portSpec string

	switch len(args) {
	case 2:
		service = args[0]
		portSpec = args[1]
	case 1:
		arg := args[0]
		if strings.Contains(arg, ":") || isNumeric(arg) {
			portSpec = arg
		} else {
			service = arg
		}
	case 0:
		// No positional args provided, relies on flags
	default:
		return "", "", 0, 0, errors.New("too many arguments provided")
	}

	if portSpec != "" {
		lp, cp, err := parsePortSpec(portSpec)
		if err != nil {
			return "", "", 0, 0, err
		}
		localPort = lp
		containerPort = cp
	}

	if service == "" && task == "" {
		return "", "", 0, 0, errors.New("must specify a service name or --task")
	}

	if localPort <= 0 || localPort > 65535 {
		return "", "", 0, 0, fmt.Errorf("invalid local port %d: must be between 1 and 65535", localPort)
	}
	if containerPort <= 0 || containerPort > 65535 {
		return "", "", 0, 0, fmt.Errorf("invalid container port %d: must be between 1 and 65535", containerPort)
	}

	return service, task, localPort, containerPort, nil
}

func init() {
	portForwardCmd.Flags().StringVar(&portForwardCluster, "cluster", "", "ECS cluster name (required)")
	portForwardCmd.Flags().StringVar(&portForwardContainer, "container", "", "Container name to forward to (defaults to first container)")
	portForwardCmd.Flags().StringVar(&portForwardTask, "task", "", "Specific task ID or ARN (defaults to a running task in the service)")
	portForwardCmd.Flags().IntVar(&portForwardLocalPort, "local-port", 0, "Local port to listen on")
	portForwardCmd.Flags().IntVar(&portForwardContainerPort, "container-port", 0, "Container port to forward traffic to")
	_ = portForwardCmd.MarkFlagRequired("cluster")
	rootCmd.AddCommand(portForwardCmd)
}
