package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// PortForwardOptions holds options for port forwarding to an ECS task container.
type PortForwardOptions struct {
	Cluster       string
	Service       string
	TaskID        string
	Container     string
	LocalPort     int
	ContainerPort int
	Profile       string
}

type sessionPluginRunnerFunc func(ctx context.Context, pluginPath string, args []string) error

var defaultSessionPluginRunner = func(ctx context.Context, pluginPath string, args []string) error {
	cmd := exec.CommandContext(ctx, pluginPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

var (
	sessionPluginRunner = defaultSessionPluginRunner
	sessionLookPath     = exec.LookPath
)

// PortForward establishes a port-forwarding session from localPort to containerPort
// on the specified task or service using AWS Systems Manager.
func (c *Client) PortForward(ctx context.Context, opts PortForwardOptions) error {
	if opts.Cluster == "" {
		return errors.New("cluster is required")
	}
	if opts.LocalPort <= 0 || opts.LocalPort > 65535 {
		return fmt.Errorf("invalid local port %d: must be between 1 and 65535", opts.LocalPort)
	}
	if opts.ContainerPort <= 0 || opts.ContainerPort > 65535 {
		return fmt.Errorf("invalid container port %d: must be between 1 and 65535", opts.ContainerPort)
	}

	taskID := opts.TaskID
	if taskID == "" {
		if opts.Service == "" {
			return errors.New("must specify either a service name or a task ID")
		}
		listOut, err := c.ecs.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       awssdk.String(opts.Cluster),
			ServiceName:   awssdk.String(opts.Service),
			DesiredStatus: types.DesiredStatusRunning,
		})
		if err != nil {
			return fmt.Errorf("listing running tasks for service %q: %w", opts.Service, err)
		}
		if len(listOut.TaskArns) == 0 {
			return fmt.Errorf("no running tasks found for service %q in cluster %q", opts.Service, opts.Cluster)
		}
		taskID = listOut.TaskArns[0]
	}

	tasksOut, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: awssdk.String(opts.Cluster),
		Tasks:   []string{taskID},
	})
	if err != nil {
		return fmt.Errorf("describing task %s: %w", taskID, err)
	}
	if len(tasksOut.Tasks) == 0 {
		return fmt.Errorf("task %s not found in cluster %q", taskID, opts.Cluster)
	}
	task := tasksOut.Tasks[0]

	if len(task.Containers) == 0 {
		return fmt.Errorf("task %s has no containers", taskID)
	}

	var targetContainer *types.Container
	if opts.Container != "" {
		for i := range task.Containers {
			if awssdk.ToString(task.Containers[i].Name) == opts.Container {
				targetContainer = &task.Containers[i]
				break
			}
		}
		if targetContainer == nil {
			return fmt.Errorf("container %q not found in task %s", opts.Container, taskID)
		}
	} else {
		targetContainer = &task.Containers[0]
	}

	runtimeID := awssdk.ToString(targetContainer.RuntimeId)
	if runtimeID == "" {
		status := awssdk.ToString(targetContainer.LastStatus)
		if status == "" {
			status = "UNKNOWN"
		}
		return fmt.Errorf("container %q in task %s has no runtime ID (status: %s); ensure the container is running and ECS Exec is enabled", awssdk.ToString(targetContainer.Name), taskID, status)
	}

	cleanTaskID := taskID
	if idx := strings.LastIndex(cleanTaskID, "/"); idx != -1 {
		cleanTaskID = cleanTaskID[idx+1:]
	}
	clusterName := opts.Cluster
	if idx := strings.LastIndex(clusterName, "/"); idx != -1 {
		clusterName = clusterName[idx+1:]
	}

	target := fmt.Sprintf("ecs:%s_%s_%s", clusterName, cleanTaskID, runtimeID)

	pluginPath, err := sessionLookPath("session-manager-plugin")
	if err != nil {
		return fmt.Errorf("session-manager-plugin not found in PATH.\nPlease install it: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}

	if c.ssm == nil {
		return errors.New("SSM client is unavailable on this AWS client")
	}

	input := &ssm.StartSessionInput{
		Target:       awssdk.String(target),
		DocumentName: awssdk.String("AWS-StartPortForwardingSession"),
		Parameters: map[string][]string{
			"portNumber":      {strconv.Itoa(opts.ContainerPort)},
			"localPortNumber": {strconv.Itoa(opts.LocalPort)},
		},
	}

	out, err := c.ssm.StartSession(ctx, input)
	if err != nil {
		return fmt.Errorf("starting SSM port-forwarding session: %w", err)
	}

	respJSON, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling session response: %w", err)
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshaling session input: %w", err)
	}

	endpoint := fmt.Sprintf("https://ssm.%s.amazonaws.com", c.region)

	fmt.Printf("Forwarding from 127.0.0.1:%d -> container port %d (task: %s, container: %s)\n",
		opts.LocalPort, opts.ContainerPort, cleanTaskID, awssdk.ToString(targetContainer.Name))
	fmt.Println("Press Ctrl+C to stop port forwarding.")

	return sessionPluginRunner(ctx, pluginPath, []string{
		string(respJSON),
		c.region,
		"StartSession",
		opts.Profile,
		string(inputJSON),
		endpoint,
	})
}
