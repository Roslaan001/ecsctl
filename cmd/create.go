package cmd

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/roslaan001/ecsctl/pkg/aws"
	"github.com/roslaan001/ecsctl/pkg/config"
	"github.com/roslaan001/ecsctl/pkg/localconfig"
	"github.com/roslaan001/ecsctl/pkg/state"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// createCmd is the parent for all "create" subcommands.
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create ECS resources",
}

// --- create cluster flags ---
var (
	createClusterFile              string
	createClusterName              string
	createClusterRegion            string
	createClusterCapacityProviders []string
	createClusterTags              []string // key=value pairs
)

// create cluster
var createClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Create an ECS cluster",
	Example: `  # From a YAML file
  ecsctl create cluster -f cluster.yaml

  # Inline flags
  ecsctl create cluster --name my-cluster --region eu-west-2
  ecsctl create cluster --name my-cluster --region eu-west-2 --capacity-providers FARGATE,FARGATE_SPOT`,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		var cfg *config.ClusterConfig
		var err error

		if createClusterFile != "" {
			// Load from YAML file
			cfg, err = config.LoadClusterConfig(createClusterFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
		} else {
			// Build config from flags
			if createClusterName == "" {
				return fmt.Errorf("--name is required (or use -f to provide a config file)")
			}
			cfg = &config.ClusterConfig{
				Name:              createClusterName,
				Region:            createClusterRegion,
				CapacityProviders: createClusterCapacityProviders,
				Tags:              parseTags(createClusterTags),
			}
		}

		// --region flag on root cmd takes precedence over config file region
		resolvedRegion := region
		if resolvedRegion == "" {
			resolvedRegion = cfg.Region
		}
		session, err := beginStateSession(context.Background())
		if err != nil {
			return fmt.Errorf("locking remote state: %w", err)
		}
		if session != nil {
			defer closeStateSessionOnReturn(session, &runErr, "releasing remote state lock")
		}

		client, err := aws.NewECSClient(context.Background(), resolvedRegion, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}
		if resolvedRegion == "" {
			resolvedRegion = client.Region()
		}

		fmt.Printf("Creating cluster %q in %s...\n", cfg.Name, resolvedRegion)
		if err := client.CreateCluster(context.Background(), cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Cluster %q created successfully.\n", cfg.Name)

		// Write to remote state if a context is configured
		if session != nil {
			if err := recordClusterState(session, client, cfg, resolvedRegion); err != nil {
				return fmt.Errorf("cluster created but remote state was not updated: %w", err)
			}
		}

		return nil
	},
}

// --- create service flags ---
var (
	createServiceFile               string
	createServiceName               string
	createServiceCluster            string
	createServiceTaskDefinition     string
	createServiceLaunchType         string
	createServiceSchedulingStrategy string
	createServiceDesiredCount       int32
	createServiceSubnets            []string
	createServiceSecurityGroups     []string
	createServicePublicIP           string
	createServiceTags               []string
	createServiceWait               bool
)

// create service
var createServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Create an ECS service",
	Example: `  # From a YAML file
  ecsctl create service -f service.yaml

  # Inline flags
  ecsctl create service --name my-service --cluster my-cluster --task-definition my-task:3 \
    --subnets subnet-abc123 --security-groups sg-abc123`,
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		var cfg *config.ServiceConfig
		var err error

		if createServiceFile != "" {
			cfg, err = config.LoadServiceConfig(createServiceFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
		} else {
			// Validate required inline flags
			if createServiceName == "" {
				return fmt.Errorf("--name is required (or use -f to provide a config file)")
			}
			if createServiceCluster == "" {
				return fmt.Errorf("--cluster is required")
			}
			if createServiceTaskDefinition == "" {
				return fmt.Errorf("--task-definition is required")
			}

			schedulingStrategy := createServiceSchedulingStrategy
			if schedulingStrategy == "" {
				schedulingStrategy = "REPLICA"
			}
			if schedulingStrategy != "REPLICA" && schedulingStrategy != "DAEMON" {
				return fmt.Errorf("--scheduling-strategy must be REPLICA or DAEMON")
			}
			desiredCount := createServiceDesiredCount
			if schedulingStrategy == "DAEMON" {
				if cmd.Flags().Changed("desired-count") && desiredCount > 0 {
					return fmt.Errorf("--desired-count cannot be set for DAEMON services")
				}
				desiredCount = 0
			}
			cfg = &config.ServiceConfig{
				Name:               createServiceName,
				Cluster:            createServiceCluster,
				TaskDefinition:     createServiceTaskDefinition,
				LaunchType:         createServiceLaunchType,
				SchedulingStrategy: schedulingStrategy,
				DesiredCount:       desiredCount,
				Tags:               parseTags(createServiceTags),
			}

			if len(createServiceSubnets) > 0 || len(createServiceSecurityGroups) > 0 {
				cfg.NetworkConfig = &config.NetworkConfig{
					Subnets:        createServiceSubnets,
					SecurityGroups: createServiceSecurityGroups,
					AssignPublicIP: createServicePublicIP,
				}
			}
		}
		session, err := beginStateSession(context.Background())
		if err != nil {
			return fmt.Errorf("locking remote state: %w", err)
		}
		if session != nil {
			defer closeStateSessionOnReturn(session, &runErr, "releasing remote state lock")
		}

		client, err := aws.NewECSClient(context.Background(), region, profile)
		if err != nil {
			return fmt.Errorf("creating AWS client: %w", err)
		}

		fmt.Printf("Creating service %q in cluster %q...\n", cfg.Name, cfg.Cluster)
		if err := client.CreateService(context.Background(), cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}

		fmt.Printf("✓ Service %q created successfully.\n", cfg.Name)
		if session != nil {
			if err := recordServiceState(session, client, cfg, region); err != nil {
				return fmt.Errorf("service created but remote state was not updated: %w", err)
			}
		}

		if createServiceWait {
			fmt.Println("Waiting for service to reach steady state...")
			if err := client.WaitForServiceStable(context.Background(), cfg.Cluster, cfg.Name); err != nil {
				fmt.Fprintf(os.Stderr, "Service unstable: %v\n", err)
				return err
			}
			fmt.Printf("✓ Service %q is stable.\n", cfg.Name)
		}

		// Write to remote state if a context is configured
		return nil
	},
}

// parseTags converts ["key=value", ...] flag values into a map.
func parseTags(tags []string) map[string]string {
	result := make(map[string]string, len(tags))
	for _, t := range tags {
		for i, c := range t {
			if c == '=' {
				result[t[:i]] = t[i+1:]
				break
			}
		}
	}
	return result
}

func init() {
	// create cluster flags
	createClusterCmd.Flags().StringVarP(&createClusterFile, "file", "f", "", "Path to cluster config YAML")
	createClusterCmd.Flags().StringVar(&createClusterName, "name", "", "Cluster name")
	createClusterCmd.Flags().StringVar(&createClusterRegion, "region", "", "AWS region (e.g. eu-west-2)")
	createClusterCmd.Flags().StringSliceVar(&createClusterCapacityProviders, "capacity-providers", nil, "Capacity providers, e.g. FARGATE,FARGATE_SPOT (optional)")
	createClusterCmd.Flags().StringArrayVar(&createClusterTags, "tags", nil, "Tags as key=value pairs (repeatable: --tags env=prod --tags team=platform)")

	// create service flags
	createServiceCmd.Flags().StringVarP(&createServiceFile, "file", "f", "", "Path to service config YAML")
	createServiceCmd.Flags().StringVar(&createServiceName, "name", "", "Service name")
	createServiceCmd.Flags().StringVar(&createServiceCluster, "cluster", "", "ECS cluster name")
	createServiceCmd.Flags().StringVar(&createServiceTaskDefinition, "task-definition", "", "Task definition family:revision (e.g. my-task:3)")
	createServiceCmd.Flags().StringVar(&createServiceLaunchType, "launch-type", "FARGATE", "Launch type: FARGATE or EC2")
	createServiceCmd.Flags().StringVar(&createServiceSchedulingStrategy, "scheduling-strategy", "REPLICA", "Scheduling strategy: REPLICA or DAEMON")
	createServiceCmd.Flags().Int32Var(&createServiceDesiredCount, "desired-count", 1, "Desired task count")
	createServiceCmd.Flags().StringSliceVar(&createServiceSubnets, "subnets", nil, "Subnet IDs (comma-separated)")
	createServiceCmd.Flags().StringSliceVar(&createServiceSecurityGroups, "security-groups", nil, "Security group IDs (comma-separated)")
	createServiceCmd.Flags().StringVar(&createServicePublicIP, "assign-public-ip", "ENABLED", "Assign public IP: ENABLED or DISABLED")
	createServiceCmd.Flags().StringArrayVar(&createServiceTags, "tags", nil, "Tags as key=value pairs (repeatable)")
	createServiceCmd.Flags().BoolVar(&createServiceWait, "wait", false, "Wait for the service to reach steady state")

	createCmd.AddCommand(createClusterCmd)
	createCmd.AddCommand(createServiceCmd)
}

func currentUsername() string {
	u, _ := user.Current()
	if u == nil {
		return "unknown"
	}
	return u.Username
}

// stateSession holds the remote state lock across a resource operation and its
// state update, serializing ecsctl mutating commands that share a context.
type stateSession struct {
	backend *state.Backend
	current *state.State
	ctx     context.Context
}

func beginStateSession(ctx context.Context) (*stateSession, error) {
	localCfg, err := localconfig.Load()
	if err != nil {
		return nil, err
	}
	if localCfg.CurrentContext == "" && stateContext == "" {
		return nil, nil
	}
	_, activeCtx, err := localCfg.GetActiveContext(stateContext)
	if err != nil {
		return nil, err
	}
	backend, err := state.NewBackend(ctx, activeCtx.Bucket, activeCtx.Key, activeCtx.Region, activeCtx.Profile, activeCtx.KmsKeyID)
	if err != nil {
		return nil, err
	}
	if err := backend.Lock(ctx); err != nil {
		return nil, err
	}
	current, err := backend.Load(ctx)
	if err != nil {
		if unlockErr := backend.Unlock(ctx); unlockErr != nil {
			return nil, fmt.Errorf("loading state: %v; releasing state lock: %w", err, unlockErr)
		}
		return nil, err
	}
	return &stateSession{backend: backend, current: current, ctx: ctx}, nil
}

func (s *stateSession) Update(fn func(*state.State)) error {
	if s == nil {
		return nil
	}
	fn(s.current)
	return s.backend.Save(s.ctx, s.current)
}

func (s *stateSession) Close() error {
	if s == nil {
		return nil
	}
	return s.backend.Unlock(s.ctx)
}

func closeStateSessionOnReturn(session *stateSession, runErr *error, operation string) {
	if session == nil {
		return
	}
	if err := session.Close(); err != nil {
		if *runErr == nil {
			*runErr = fmt.Errorf("%s: %w", operation, err)
		} else {
			*runErr = fmt.Errorf("%w; %s: %v", *runErr, operation, err)
		}
	}
}

func recordClusterState(s *stateSession, client *aws.Client, cfg *config.ClusterConfig, region string) error {
	if region == "" {
		region = client.Region()
	}
	storedConfig := *cfg
	if storedConfig.Region == "" {
		storedConfig.Region = region
	}
	serialized, err := yaml.Marshal(&storedConfig)
	if err != nil {
		return fmt.Errorf("serializing cluster config for remote state: %w", err)
	}
	arn, err := client.ClusterARN(context.Background(), cfg.Name)
	if err != nil {
		return fmt.Errorf("reading cluster identity for remote state: %w", err)
	}
	resource := state.Resource{
		Type: state.ResourceTypeCluster, Name: cfg.Name, ARN: arn,
		Region: region, CreatedBy: currentUsername(), CreatedAt: time.Now().UTC(), Configuration: string(serialized),
	}
	return s.Update(func(st *state.State) {
		st.AddResource(resource)
	})
}

func recordServiceState(s *stateSession, client *aws.Client, cfg *config.ServiceConfig, region string) error {
	if region == "" {
		region = client.Region()
	}
	serialized, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializing service config for remote state: %w", err)
	}
	arn, err := client.ServiceARN(context.Background(), cfg.Cluster, cfg.Name)
	if err != nil {
		return fmt.Errorf("reading service identity for remote state: %w", err)
	}
	resource := state.Resource{
		Type: state.ResourceTypeService, Name: cfg.Name, ARN: arn, Cluster: cfg.Cluster,
		Region: region, CreatedBy: currentUsername(), CreatedAt: time.Now().UTC(), Configuration: string(serialized),
	}
	return s.Update(func(st *state.State) {
		st.AddResource(resource)
	})
}

func recordExpressState(s *stateSession, client *aws.Client, cfg *config.ExpressServiceConfig, arn, region string) error {
	if region == "" {
		region = client.Region()
	}
	serialized, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serializing Express config for remote state: %w", err)
	}
	resource := &state.Resource{
		Type: state.ResourceTypeExpressService, Name: cfg.ServiceName, ARN: arn, Cluster: cfg.Cluster,
		Region: region, CreatedBy: currentUsername(), CreatedAt: time.Now().UTC(), Configuration: string(serialized),
	}
	return s.Update(func(st *state.State) {
		st.AddResource(*resource)
	})
}

func updateTrackedServiceConfig(session *stateSession, client *aws.Client, cluster, service string, update func(*config.ServiceConfig)) error {
	if session == nil {
		return nil
	}
	arn, err := client.ServiceARN(context.Background(), cluster, service)
	if err != nil {
		return err
	}
	for _, resource := range session.current.Resources {
		if resource.Type != state.ResourceTypeService || resource.Name != service || resource.Cluster != cluster || (resource.ARN != "" && resource.ARN != arn) {
			continue
		}
		if resource.Configuration == "" {
			return fmt.Errorf("tracked service %q has no saved configuration to update", service)
		}
		var cfg config.ServiceConfig
		if err := yaml.Unmarshal([]byte(resource.Configuration), &cfg); err != nil {
			return fmt.Errorf("parsing tracked service configuration: %w", err)
		}
		var fields map[string]yaml.Node
		if err := yaml.Unmarshal([]byte(resource.Configuration), &fields); err != nil {
			return fmt.Errorf("parsing tracked service configuration: %w", err)
		}
		_, cfg.DesiredCountConfigured = fields["desiredCount"]
		_, cfg.HealthCheckGraceConfigured = fields["healthCheckGracePeriodSeconds"]
		_, cfg.LaunchTypeConfigured = fields["launchType"]
		update(&cfg)
		serialized, err := yaml.Marshal(&cfg)
		if err != nil {
			return fmt.Errorf("serializing tracked service configuration: %w", err)
		}
		resource.ARN = arn
		resource.Region = client.Region()
		resource.Configuration = string(serialized)
		return session.Update(func(st *state.State) {
			st.AddResource(resource)
		})
	}
	return nil
}
