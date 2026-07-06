package aws

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
	"github.com/roslaan001/ecsctl/pkg/state"
)

// ecsIface is the subset of the ECS SDK client used by Client.
// Defining it as an interface allows tests to inject a mock.
type ecsIface interface {
	CreateCluster(ctx context.Context, params *ecs.CreateClusterInput, optFns ...func(*ecs.Options)) (*ecs.CreateClusterOutput, error)
	DeleteCluster(ctx context.Context, params *ecs.DeleteClusterInput, optFns ...func(*ecs.Options)) (*ecs.DeleteClusterOutput, error)
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	CreateService(ctx context.Context, params *ecs.CreateServiceInput, optFns ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error)
	DeleteService(ctx context.Context, params *ecs.DeleteServiceInput, optFns ...func(*ecs.Options)) (*ecs.DeleteServiceOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	RegisterTaskDefinition(ctx context.Context, params *ecs.RegisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	ExecuteCommand(ctx context.Context, params *ecs.ExecuteCommandInput, optFns ...func(*ecs.Options)) (*ecs.ExecuteCommandOutput, error)
}

// Client wraps the AWS ECS and CloudWatch Logs SDK clients.
type Client struct {
	ecs  ecsIface
	logs *cloudwatchlogs.Client
}

// LogsOptions holds options for fetching logs.
type LogsOptions struct {
	Cluster       string
	Service       string
	ContainerName string
	Tail          bool
	TailLines     bool
}

// ExecOptions holds options for exec into a task.
type ExecOptions struct {
	Cluster   string
	Service   string
	TaskID    string
	Container string
	Command   string
}

// NewECSClient creates a new AWS client using the given region and profile.
// If region or profile are empty, the SDK falls back to env vars / shared config.
func NewECSClient(ctx context.Context, region, profile string) (*Client, error) {
	var opts []func(*config.LoadOptions) error

	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &Client{
		ecs:  ecs.NewFromConfig(cfg),
		logs: cloudwatchlogs.NewFromConfig(cfg),
	}, nil
}

// CreateCluster creates an ECS cluster from config.
// Returns an error if a cluster with the same name already exists and is ACTIVE.
func (c *Client) CreateCluster(ctx context.Context, cfg *ecscfg.ClusterConfig) error {
	// Existence check — use AWS as source of truth
	existing, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{cfg.Name},
	})
	if err != nil {
		return fmt.Errorf("checking for existing cluster: %w", err)
	}
	for _, cl := range existing.Clusters {
		if aws.ToString(cl.ClusterName) == cfg.Name && cl.Status != nil && *cl.Status == "ACTIVE" {
			return fmt.Errorf("cluster %q already exists (status: ACTIVE) — use 'ecsctl delete cluster %s' to remove it first", cfg.Name, cfg.Name)
		}
	}

	input := &ecs.CreateClusterInput{
		ClusterName: aws.String(cfg.Name),
		Tags:        toECSTags(cfg.Tags),
	}

	// Only set capacity providers when explicitly specified — omitting them avoids
	// the service-linked role assumption that can fail on fresh accounts.
	if len(cfg.CapacityProviders) > 0 {
		input.CapacityProviders = cfg.CapacityProviders
	}

	_, err = c.ecs.CreateCluster(ctx, input)
	return err
}

// DeleteCluster deletes an ECS cluster by name.
// Returns an error if the cluster has active services unless force is true,
// in which case all services are drained and deleted first.
func (c *Client) DeleteCluster(ctx context.Context, clusterName string, force bool) error {
	// Check for active services
	listOut, err := c.ecs.ListServices(ctx, &ecs.ListServicesInput{
		Cluster: aws.String(clusterName),
	})
	if err != nil {
		return fmt.Errorf("listing services in cluster: %w", err)
	}

	if len(listOut.ServiceArns) > 0 {
		if !force {
			// Describe services to show names
			descOut, _ := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: listOut.ServiceArns,
			})
			fmt.Printf("✗ Cluster %q has %d active service(s):\n", clusterName, len(listOut.ServiceArns))
			for _, svc := range descOut.Services {
				fmt.Printf("  - %s (%d running tasks)\n", aws.ToString(svc.ServiceName), svc.RunningCount)
			}
			return fmt.Errorf("refusing to delete cluster with active services — delete them first or use --force to delete all services automatically")
		}

		// --force: drain and delete all services first
		fmt.Printf("Draining and deleting %d service(s) in cluster %q...\n", len(listOut.ServiceArns), clusterName)
		descOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: listOut.ServiceArns,
		})
		if err != nil {
			return fmt.Errorf("describing services: %w", err)
		}
		for _, svc := range descOut.Services {
			svcName := aws.ToString(svc.ServiceName)
			fmt.Printf("  Deleting service %q...\n", svcName)
			if err := c.DeleteService(ctx, clusterName, svcName); err != nil {
				return fmt.Errorf("deleting service %q: %w", svcName, err)
			}
		}
	}

	_, err = c.ecs.DeleteCluster(ctx, &ecs.DeleteClusterInput{
		Cluster: aws.String(clusterName),
	})
	return err
}

// CreateService creates an ECS service from config.
// Returns an error if a service with the same name already exists in the cluster.
func (c *Client) CreateService(ctx context.Context, cfg *ecscfg.ServiceConfig) error {
	// Existence check
	existing, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Cluster),
		Services: []string{cfg.Name},
	})
	if err != nil {
		return fmt.Errorf("checking for existing service: %w", err)
	}
	for _, svc := range existing.Services {
		if aws.ToString(svc.ServiceName) == cfg.Name && aws.ToString(svc.Status) == "ACTIVE" {
			return fmt.Errorf("service %q already exists in cluster %q — use 'ecsctl delete service %s --cluster %s' to remove it first", cfg.Name, cfg.Cluster, cfg.Name, cfg.Cluster)
		}
	}

	input := &ecs.CreateServiceInput{
		Cluster:        aws.String(cfg.Cluster),
		ServiceName:    aws.String(cfg.Name),
		TaskDefinition: aws.String(cfg.TaskDefinition),
		DesiredCount:   aws.Int32(cfg.DesiredCount),
		LaunchType:     types.LaunchType(cfg.LaunchType),
	}

	if cfg.NetworkConfig != nil {
		input.NetworkConfiguration = &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        cfg.NetworkConfig.Subnets,
				SecurityGroups: cfg.NetworkConfig.SecurityGroups,
				AssignPublicIp: types.AssignPublicIp(cfg.NetworkConfig.AssignPublicIP),
			},
		}
	}

	_, err = c.ecs.CreateService(ctx, input)
	return err
}

// DeleteService drains and deletes an ECS service.
func (c *Client) DeleteService(ctx context.Context, clusterName, serviceName string) error {
	// Scale to 0 first so tasks are drained cleanly
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(clusterName),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(0),
	})
	if err != nil {
		return fmt.Errorf("draining service: %w", err)
	}

	_, err = c.ecs.DeleteService(ctx, &ecs.DeleteServiceInput{
		Cluster: aws.String(clusterName),
		Service: aws.String(serviceName),
		Force:   aws.Bool(true),
	})
	return err
}

// ScaleService updates the desired count of an ECS service.
func (c *Client) ScaleService(ctx context.Context, clusterName, serviceName string, desired int32) error {
	_, err := c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:      aws.String(clusterName),
		Service:      aws.String(serviceName),
		DesiredCount: aws.Int32(desired),
	})
	return err
}

// DeployService registers a new task definition revision with the updated image
// and updates the service to use it. Returns the new task definition ARN.
func (c *Client) DeployService(ctx context.Context, clusterName, serviceName, containerName, image string) (string, error) {
	// 1. Describe the current service to get the task definition ARN
	svcOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil || len(svcOut.Services) == 0 {
		return "", fmt.Errorf("describing service: %w", err)
	}

	currentTaskDef := aws.ToString(svcOut.Services[0].TaskDefinition)

	// 2. Describe the task definition
	tdOut, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(currentTaskDef),
	})
	if err != nil {
		return "", fmt.Errorf("describing task definition: %w", err)
	}

	td := tdOut.TaskDefinition
	containers := td.ContainerDefinitions

	// 3. Update the target container's image
	updated := false
	for i := range containers {
		if containerName == "" || aws.ToString(containers[i].Name) == containerName {
			containers[i].Image = aws.String(image)
			updated = true
			break
		}
	}
	if !updated {
		return "", fmt.Errorf("container %q not found in task definition", containerName)
	}

	// 4. Register a new task definition revision
	newTD, err := c.ecs.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family:                  td.Family,
		ContainerDefinitions:    containers,
		Cpu:                     td.Cpu,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		RequiresCompatibilities: td.RequiresCompatibilities,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		TaskRoleArn:             td.TaskRoleArn,
		Volumes:                 td.Volumes,
	})
	if err != nil {
		return "", fmt.Errorf("registering task definition: %w", err)
	}

	newTaskDefARN := aws.ToString(newTD.TaskDefinition.TaskDefinitionArn)

	// 5. Update the service to use the new task definition
	_, err = c.ecs.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:        aws.String(clusterName),
		Service:        aws.String(serviceName),
		TaskDefinition: aws.String(newTaskDefARN),
	})
	if err != nil {
		return "", err
	}
	return newTaskDefARN, nil
}

// WaitForDeployment polls the service deployments until the deployment using
// newTaskDefARN reaches a PRIMARY/COMPLETED state with runningCount == desiredCount,
// printing progress dots every 5 seconds. Times out after 10 minutes.
func (c *Client) WaitForDeployment(ctx context.Context, clusterName, serviceName, newTaskDefARN string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for deployment to complete (10m)")
		case <-ticker.C:
			svcOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: []string{serviceName},
			})
			if err != nil || len(svcOut.Services) == 0 {
				return fmt.Errorf("describing service: %w", err)
			}

			svc := svcOut.Services[0]

			// Find the deployment for our new task definition
			for _, d := range svc.Deployments {
				if aws.ToString(d.TaskDefinition) != newTaskDefARN {
					continue
				}

				fmt.Printf("  [%s] desired=%d running=%d pending=%d\n",
					string(d.RolloutState),
					d.DesiredCount,
					d.RunningCount,
					d.PendingCount,
				)

				switch d.RolloutState {
				case types.DeploymentRolloutStateCompleted:
					return nil
				case types.DeploymentRolloutStateFailed:
					reason := aws.ToString(d.RolloutStateReason)
					return fmt.Errorf("deployment failed: %s", reason)
				}
			}
		}
	}
}

// WaitForServiceStable polls until a service has runningCount == desiredCount
// and no pending tasks. Times out after 10 minutes.
func (c *Client) WaitForServiceStable(ctx context.Context, clusterName, serviceName string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*60*time.Second)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for service to stabilise (10m)")
		case <-ticker.C:
			out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: []string{serviceName},
			})
			if err != nil || len(out.Services) == 0 {
				continue
			}
			svc := out.Services[0]
			fmt.Printf("  desired=%d running=%d pending=%d\n",
				svc.DesiredCount, svc.RunningCount, svc.PendingCount)
			if svc.RunningCount == svc.DesiredCount && svc.PendingCount == 0 {
				return nil
			}
		}
	}
}

// FetchLogs retrieves CloudWatch logs for a service. If opts.Tail is true,
// it streams continuously (--follow), polling every 2 seconds for new events.
func (c *Client) FetchLogs(ctx context.Context, opts LogsOptions, tail int) error {
	// Resolve log group and stream prefix from the task definition
	svcOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(opts.Cluster),
		Services: []string{opts.Service},
	})
	if err != nil || len(svcOut.Services) == 0 {
		return fmt.Errorf("describing service: %w", err)
	}

	tdOut, err := c.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: svcOut.Services[0].TaskDefinition,
	})
	if err != nil {
		return fmt.Errorf("describing task definition: %w", err)
	}

	containers := tdOut.TaskDefinition.ContainerDefinitions
	if len(containers) == 0 {
		return fmt.Errorf("no containers found in task definition")
	}

	// Pick the target container
	container := containers[0]
	for _, cont := range containers {
		if opts.ContainerName != "" && aws.ToString(cont.Name) == opts.ContainerName {
			container = cont
			break
		}
	}

	if container.LogConfiguration == nil {
		return fmt.Errorf("container %q has no log configuration", aws.ToString(container.Name))
	}

	logGroup := container.LogConfiguration.Options["awslogs-group"]
	streamPrefix := container.LogConfiguration.Options["awslogs-stream-prefix"]
	containerName := aws.ToString(container.Name)

	// Find a running task to get the task ID for the stream name
	listOut, err := c.ecs.ListTasks(ctx, &ecs.ListTasksInput{
		Cluster:     aws.String(opts.Cluster),
		ServiceName: aws.String(opts.Service),
	})
	if err != nil || len(listOut.TaskArns) == 0 {
		return fmt.Errorf("no running tasks found for service %q", opts.Service)
	}
	// Extract task ID from the last segment of the ARN (after the final '/')
	taskARN := listOut.TaskArns[0]
	taskID := taskARN
	if parts := strings.Split(taskARN, "/"); len(parts) > 1 {
		taskID = parts[len(parts)-1]
	}

	logStreamName := fmt.Sprintf("%s/%s/%s", streamPrefix, containerName, taskID)

	if !opts.Tail {
		// One-shot fetch
		out, err := c.logs.GetLogEvents(ctx, &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(logGroup),
			LogStreamName: aws.String(logStreamName),
			Limit:         aws.Int32(int32(tail)),
			StartFromHead: aws.Bool(false),
		})
		if err != nil {
			return fmt.Errorf("fetching logs: %w", err)
		}
		for _, event := range out.Events {
			fmt.Println(aws.ToString(event.Message))
		}
		return nil
	}

	// --follow: stream continuously using nextForwardToken
	fmt.Fprintf(os.Stderr, "Streaming logs for %s/%s (Ctrl+C to stop)...\n", opts.Service, containerName)
	var nextToken *string
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			input := &cloudwatchlogs.GetLogEventsInput{
				LogGroupName:  aws.String(logGroup),
				LogStreamName: aws.String(logStreamName),
				StartFromHead: aws.Bool(false),
			}
			if nextToken != nil {
				input.NextToken = nextToken
			} else {
				// First call — start from the tail
				input.Limit = aws.Int32(int32(tail))
			}

			out, err := c.logs.GetLogEvents(ctx, input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				continue
			}
			for _, event := range out.Events {
				fmt.Println(aws.ToString(event.Message))
			}
			// Only advance the token when new events came in; otherwise
			// CloudWatch returns the same token and we'd loop forever.
			if out.NextForwardToken != nil && (nextToken == nil || *out.NextForwardToken != *nextToken) {
				nextToken = out.NextForwardToken
			}
		}
	}
}

// ExecInTask opens an interactive shell session in a running ECS task.
// Requires ECS Exec to be enabled on the service and task.
func (c *Client) ExecInTask(ctx context.Context, opts ExecOptions) error {
	taskID := opts.TaskID

	// If no task ID provided, find a running task for the service
	if taskID == "" {
		listOut, err := c.ecs.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:     aws.String(opts.Cluster),
			ServiceName: aws.String(opts.Service),
		})
		if err != nil || len(listOut.TaskArns) == 0 {
			return fmt.Errorf("no running tasks found for service %q", opts.Service)
		}
		taskID = listOut.TaskArns[0]
	}

	container := opts.Container
	if container == "" {
		// Resolve container name from task definition
		tasksOut, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(opts.Cluster),
			Tasks:   []string{taskID},
		})
		if err != nil || len(tasksOut.Tasks) == 0 {
			return fmt.Errorf("describing task: %w", err)
		}
		if len(tasksOut.Tasks[0].Containers) > 0 {
			container = aws.ToString(tasksOut.Tasks[0].Containers[0].Name)
		}
	}

	out, err := c.ecs.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(opts.Cluster),
		Task:        aws.String(taskID),
		Container:   aws.String(container),
		Command:     aws.String(opts.Command),
		Interactive: true,
	})
	if err != nil {
		return fmt.Errorf("executing command: %w", err)
	}

	// The session token is used with the SSM session manager plugin
	fmt.Printf("Session started: %s\n", aws.ToString(out.Session.SessionId))
	fmt.Println("Note: pipe this through the AWS SSM session-manager-plugin for full interactivity.")
	return nil
}

// ListClusters lists all ECS clusters in the account/region and prints them.
func (c *Client) ListClusters(ctx context.Context) error {
	var nextToken *string
	fmt.Printf("%-40s %-10s %s\n", "NAME", "STATUS", "ARN")
	fmt.Println("--------------------------------------------------------------------------------")

	for {
		listOut, err := c.ecs.ListClusters(ctx, &ecs.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("listing clusters: %w", err)
		}
		if len(listOut.ClusterArns) == 0 {
			fmt.Println("No clusters found.")
			return nil
		}

		descOut, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: listOut.ClusterArns,
		})
		if err != nil {
			return fmt.Errorf("describing clusters: %w", err)
		}

		for _, cl := range descOut.Clusters {
			status := ""
			if cl.Status != nil {
				status = *cl.Status
			}
			fmt.Printf("%-40s %-10s %s\n",
				aws.ToString(cl.ClusterName),
				status,
				aws.ToString(cl.ClusterArn),
			)
		}

		if listOut.NextToken == nil {
			break
		}
		nextToken = listOut.NextToken
	}
	return nil
}

// ListServices lists all services in a given ECS cluster and prints them.
func (c *Client) ListServices(ctx context.Context, clusterName string) error {
	var nextToken *string
	fmt.Printf("%-40s %-10s %-8s %-8s %s\n", "NAME", "STATUS", "DESIRED", "RUNNING", "TASK DEFINITION")
	fmt.Println("--------------------------------------------------------------------------------")

	for {
		listOut, err := c.ecs.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:   aws.String(clusterName),
			NextToken: nextToken,
		})
		if err != nil {
			return fmt.Errorf("listing services: %w", err)
		}
		if len(listOut.ServiceArns) == 0 {
			fmt.Println("No services found.")
			return nil
		}

		descOut, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(clusterName),
			Services: listOut.ServiceArns,
		})
		if err != nil {
			return fmt.Errorf("describing services: %w", err)
		}

		for _, svc := range descOut.Services {
			fmt.Printf("%-40s %-10s %-8d %-8d %s\n",
				aws.ToString(svc.ServiceName),
				aws.ToString(svc.Status),
				svc.DesiredCount,
				svc.RunningCount,
				aws.ToString(svc.TaskDefinition),
			)
		}

		if listOut.NextToken == nil {
			break
		}
		nextToken = listOut.NextToken
	}
	return nil
}

// PrintClusterDetail prints a detailed description of an ECS cluster.
func (c *Client) PrintClusterDetail(ctx context.Context, clusterName string) error {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
		Include:  []types.ClusterField{types.ClusterFieldTags, types.ClusterFieldStatistics},
	})
	if err != nil {
		return fmt.Errorf("describing cluster: %w", err)
	}
	if len(out.Clusters) == 0 {
		return fmt.Errorf("cluster %q not found", clusterName)
	}

	cl := out.Clusters[0]

	fmt.Printf("Name:       %s\n", aws.ToString(cl.ClusterName))
	fmt.Printf("ARN:        %s\n", aws.ToString(cl.ClusterArn))
	fmt.Printf("Status:     %s\n", aws.ToString(cl.Status))
	fmt.Printf("\nTask Counts:\n")
	fmt.Printf("  Running:    %d\n", cl.RunningTasksCount)
	fmt.Printf("  Pending:    %d\n", cl.PendingTasksCount)
	fmt.Printf("  Active Services: %d\n", cl.ActiveServicesCount)
	fmt.Printf("  Registered Instances: %d\n", cl.RegisteredContainerInstancesCount)

	if len(cl.CapacityProviders) > 0 {
		fmt.Printf("\nCapacity Providers:\n")
		for _, cp := range cl.CapacityProviders {
			fmt.Printf("  - %s\n", cp)
		}
	}

	if len(cl.Tags) > 0 {
		fmt.Printf("\nTags:\n")
		for _, t := range cl.Tags {
			fmt.Printf("  %s = %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
		}
	}

	return nil
}

// PrintServiceDetail prints a detailed description of an ECS service.
func (c *Client) PrintServiceDetail(ctx context.Context, clusterName, serviceName string) error {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil {
		return fmt.Errorf("describing service: %w", err)
	}
	if len(out.Services) == 0 {
		return fmt.Errorf("service %q not found in cluster %q", serviceName, clusterName)
	}

	svc := out.Services[0]

	fmt.Printf("Name:           %s\n", aws.ToString(svc.ServiceName))
	fmt.Printf("ARN:            %s\n", aws.ToString(svc.ServiceArn))
	fmt.Printf("Cluster:        %s\n", aws.ToString(svc.ClusterArn))
	fmt.Printf("Status:         %s\n", aws.ToString(svc.Status))
	fmt.Printf("Launch Type:    %s\n", string(svc.LaunchType))
	fmt.Printf("Task Definition:%s\n", aws.ToString(svc.TaskDefinition))
	fmt.Printf("\nTask Counts:\n")
	fmt.Printf("  Desired:  %d\n", svc.DesiredCount)
	fmt.Printf("  Running:  %d\n", svc.RunningCount)
	fmt.Printf("  Pending:  %d\n", svc.PendingCount)

	if svc.NetworkConfiguration != nil && svc.NetworkConfiguration.AwsvpcConfiguration != nil {
		vpc := svc.NetworkConfiguration.AwsvpcConfiguration
		fmt.Printf("\nNetwork:\n")
		fmt.Printf("  Subnets:        %v\n", vpc.Subnets)
		fmt.Printf("  Security Groups:%v\n", vpc.SecurityGroups)
		fmt.Printf("  Public IP:      %s\n", string(vpc.AssignPublicIp))
	}

	if len(svc.Deployments) > 0 {
		fmt.Printf("\nDeployments:\n")
		for _, d := range svc.Deployments {
			fmt.Printf("  [%s] %s  desired=%d running=%d pending=%d\n",
				aws.ToString(d.Status),
				string(d.RolloutState),
				d.DesiredCount,
				d.RunningCount,
				d.PendingCount,
			)
		}
	}

	if len(svc.Events) > 0 {
		fmt.Printf("\nRecent Events:\n")
		limit := 5
		if len(svc.Events) < limit {
			limit = len(svc.Events)
		}
		for _, e := range svc.Events[:limit] {
			ts := ""
			if e.CreatedAt != nil {
				ts = e.CreatedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("  %s  %s\n", ts, aws.ToString(e.Message))
		}
	}

	if len(svc.Tags) > 0 {
		fmt.Printf("\nTags:\n")
		for _, t := range svc.Tags {
			fmt.Printf("  %s = %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
		}
	}

	return nil
}

// ClusterExists returns true if an ACTIVE cluster with the given name exists.
func (c *Client) ClusterExists(ctx context.Context, clusterName string) (bool, error) {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
	})
	if err != nil {
		return false, fmt.Errorf("describing cluster: %w", err)
	}
	for _, cl := range out.Clusters {
		if aws.ToString(cl.ClusterName) == clusterName && aws.ToString(cl.Status) == "ACTIVE" {
			return true, nil
		}
	}
	return false, nil
}

// ServiceExists returns true if an ACTIVE service with the given name exists in the cluster.
func (c *Client) ServiceExists(ctx context.Context, clusterName, serviceName string) (bool, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil {
		return false, fmt.Errorf("describing service: %w", err)
	}
	for _, svc := range out.Services {
		if aws.ToString(svc.ServiceName) == serviceName && aws.ToString(svc.Status) == "ACTIVE" {
			return true, nil
		}
	}
	return false, nil
}

// ReconcileService compares a desired ServiceConfig against the live service
// and applies changes where there is drift. Returns true if any changes were made.
func (c *Client) ReconcileService(ctx context.Context, cfg *ecscfg.ServiceConfig, dryRun bool) (bool, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cfg.Cluster),
		Services: []string{cfg.Name},
	})
	if err != nil || len(out.Services) == 0 {
		return false, fmt.Errorf("describing service: %w", err)
	}

	svc := out.Services[0]
	changed := false
	updateInput := &ecs.UpdateServiceInput{
		Cluster: aws.String(cfg.Cluster),
		Service: aws.String(cfg.Name),
	}

	if svc.DesiredCount != cfg.DesiredCount {
		fmt.Printf("  desiredCount: %d → %d\n", svc.DesiredCount, cfg.DesiredCount)
		updateInput.DesiredCount = aws.Int32(cfg.DesiredCount)
		changed = true
	}

	if changed {
		if dryRun {
			fmt.Printf("[dry-run] Would update service %q desiredCount to %d\n", cfg.Name, cfg.DesiredCount)
		} else {
			if _, err := c.ecs.UpdateService(ctx, updateInput); err != nil {
				return false, fmt.Errorf("updating service: %w", err)
			}
			fmt.Printf("✓ Service %q updated.\n", cfg.Name)
		}
	}

	return changed, nil
}

// PrintTasks lists running tasks in a cluster, optionally filtered by service.
func (c *Client) PrintTasks(ctx context.Context, clusterName, serviceName string) error {
	input := &ecs.ListTasksInput{
		Cluster: aws.String(clusterName),
	}
	if serviceName != "" {
		input.ServiceName = aws.String(serviceName)
	}

	listOut, err := c.ecs.ListTasks(ctx, input)
	if err != nil {
		return fmt.Errorf("listing tasks: %w", err)
	}
	if len(listOut.TaskArns) == 0 {
		fmt.Println("No running tasks found.")
		return nil
	}

	descOut, err := c.ecs.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(clusterName),
		Tasks:   listOut.TaskArns,
	})
	if err != nil {
		return fmt.Errorf("describing tasks: %w", err)
	}

	fmt.Printf("%-36s %-12s %-12s %-30s %s\n", "TASK ID", "STATUS", "LAUNCH TYPE", "STARTED AT", "TASK DEFINITION")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────────────────────")

	for _, task := range descOut.Tasks {
		// Extract short task ID from the last segment of the ARN
		taskID := aws.ToString(task.TaskArn)
		if parts := strings.Split(taskID, "/"); len(parts) > 1 {
			taskID = parts[len(parts)-1]
		}

		startedAt := ""
		if task.StartedAt != nil {
			startedAt = task.StartedAt.Format("2006-01-02 15:04:05")
		}

		// Short task definition
		td := aws.ToString(task.TaskDefinitionArn)
		if idx := len(td) - 1; idx > 0 {
			for i := idx; i >= 0; i-- {
				if td[i] == '/' {
					td = td[i+1:]
					break
				}
			}
		}

		fmt.Printf("%-36s %-12s %-12s %-30s %s\n",
			taskID,
			aws.ToString(task.LastStatus),
			string(task.LaunchType),
			startedAt,
			td,
		)
	}
	return nil
}

// DescribeClusterResource fetches a cluster from AWS and returns it as a state.Resource.
func (c *Client) DescribeClusterResource(ctx context.Context, clusterName string) (*state.Resource, error) {
	out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
	})
	if err != nil {
		return nil, fmt.Errorf("describing cluster: %w", err)
	}
	for _, cl := range out.Clusters {
		if aws.ToString(cl.ClusterName) == clusterName && aws.ToString(cl.Status) == "ACTIVE" {
			return &state.Resource{
				Type:      state.ResourceTypeCluster,
				Name:      clusterName,
				ARN:       aws.ToString(cl.ClusterArn),
				CreatedBy: "imported",
			}, nil
		}
	}
	return nil, fmt.Errorf("cluster %q not found or not ACTIVE", clusterName)
}

// DescribeServiceResource fetches a service from AWS and returns it as a state.Resource.
func (c *Client) DescribeServiceResource(ctx context.Context, clusterName, serviceName string) (*state.Resource, error) {
	out, err := c.ecs.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(clusterName),
		Services: []string{serviceName},
	})
	if err != nil {
		return nil, fmt.Errorf("describing service: %w", err)
	}
	for _, svc := range out.Services {
		if aws.ToString(svc.ServiceName) == serviceName && aws.ToString(svc.Status) == "ACTIVE" {
			return &state.Resource{
				Type:      state.ResourceTypeService,
				Name:      serviceName,
				ARN:       aws.ToString(svc.ServiceArn),
				Cluster:   clusterName,
				CreatedBy: "imported",
			}, nil
		}
	}
	return nil, fmt.Errorf("service %q not found or not ACTIVE in cluster %q", serviceName, clusterName)
}

// toECSTags converts a map of string tags to ECS Tag types.
func toECSTags(tags map[string]string) []types.Tag {
	result := make([]types.Tag, 0, len(tags))
	for k, v := range tags {
		k, v := k, v
		result = append(result, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return result
}
