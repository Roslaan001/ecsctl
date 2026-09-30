# Declarative Resource Management

`ecsctl` supports managing Amazon ECS resources declaratively using YAML configuration files. This allows you to store your infrastructure configurations in git and build GitOps workflows.

> [!TIP]
> Ready-to-use templates are available in the [examples/](https://github.com/Roslaan001/ecsctl/tree/main/examples) directory of the repository.

---

## The `apply` Command

The `apply` command creates a resource if it doesn't exist, or reconciles configured fields when they differ from the live AWS resource state. Omitted fields are left unchanged.

```bash
ecsctl apply -f <configuration-file>.yaml [flags]
```

### Dry Run
To preview changes without actually applying them to AWS, use the `--dry-run` flag. This is highly useful in CI pull requests to inspect what changes will be introduced:

```bash
ecsctl apply -f cluster.yaml --dry-run
```

### Waiting for Stability
By default, some operations are asynchronous. If you want the CLI to block and wait until the resource reaches a stable/running state (e.g. all tasks in a service are running and healthy), use the `--wait` flag:

```bash
ecsctl apply -f service.yaml --wait
```

## IDE Integration & Validation

`ecsctl` provides JSON Schemas for both Cluster and Service configurations. This allows modern editors (like VS Code, IntelliJ, and GoLand) to provide auto-completion, hover definitions, and real-time validation for your configuration files.

To enable validation, add the corresponding schema directive comment to the very top of your YAML file:

**For Clusters:**
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/Roslaan001/ecsctl/main/schemas/cluster.json
```

**For Services:**
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/Roslaan001/ecsctl/main/schemas/service.json
```

*(Note: In VS Code, this requires the Red Hat **YAML** extension).*

---

## Configuration Specifications

### 1. ECS Clusters (`cluster.yaml`)

Specify the configuration for an ECS cluster.

```yaml
name: my-cluster
region: us-east-1
capacityProviders:
  - FARGATE
  - FARGATE_SPOT
defaultCapacityProviderStrategy:
  - capacityProvider: FARGATE
    weight: 1
serviceConnectDefaultsNamespace: prod.local
tags:
  env: production
  team: platform
  project: core-api
```

#### Fields Reference
| Field | Type | Description |
|---|---|---|
| `name` | string | Name of the ECS cluster. |
| `region` | string | AWS region where the cluster will reside. |
| `capacityProviders` | list | List of capacity providers (e.g. `FARGATE`, `FARGATE_SPOT`). |
| `defaultCapacityProviderStrategy` | list | Default providers with optional `weight` and `base`. |
| `serviceConnectDefaultsNamespace` | string | Default Cloud Map namespace for Service Connect. |
| `tags` | map | Key-value pairs for resource tagging. |

---

### 2. ECS Services (`service.yaml`)

Specify the configuration for an ECS service running inside a cluster.

```yaml
name: my-service
cluster: my-cluster
taskDefinition: my-task-def:3
launchType: FARGATE
schedulingStrategy: REPLICA # REPLICA (default) or DAEMON
desiredCount: 2
network:
  subnets:
    - subnet-0bb123456789abcde
    - subnet-0cc123456789abcde
  securityGroups:
    - sg-0aa123456789abcde
  assignPublicIp: ENABLED
capacityProviderStrategy:
  - capacityProvider: FARGATE
    weight: 1
deploymentConfiguration:
  deploymentCircuitBreaker:
    enable: true
    rollback: true
loadBalancers:
  - targetGroupArn: arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/app/abc123
    containerName: web
    containerPort: 8080
autoScaling:
  minCapacity: 2
  maxCapacity: 10
  metric: CPU
  targetValue: 65
tags:
  env: production
```

#### Fields Reference
| Field | Type | Description |
|---|---|---|
| `name` | string | Name of the ECS service. |
| `cluster` | string | Name of the cluster the service will run in. |
| `taskDefinition` | string | Family and revision (e.g. `family:rev` or full ARN) of the task definition. |
| `launchType` | string | Launch type (usually `FARGATE` or `EC2`). |
| `schedulingStrategy` | string | `REPLICA` (default) or `DAEMON`. DAEMON services cannot set `desiredCount` or `autoScaling`. |
| `desiredCount` | integer | Number of tasks to keep running for a REPLICA service. |
| `network` | object | Network configuration for the tasks. |
| `network.subnets` | list | Subnet IDs where tasks will be spawned. |
| `network.securityGroups`| list | Security Group IDs to associate with the tasks. |
| `network.assignPublicIp`| string | Whether to assign public IP (`ENABLED` or `DISABLED`). |
| `capacityProviderStrategy` | list | Capacity providers, weights, and optional base task counts. |
| `deploymentController` | object | ECS rolling, CodeDeploy blue/green, or external deployment controller. |
| `deploymentConfiguration` | object | Rolling limits, circuit breaker/rollback, alarms, deployment strategy, and bake time. |
| `loadBalancers` | list | Target groups and task container/port mappings. |
| `serviceRegistries` | list | Cloud Map service registry mappings. |
| `serviceConnect` | object | Service Connect namespace, services, and client aliases. |
| `autoScaling` | object | Application Auto Scaling min/max task counts and CPU or memory target tracking. |
| `enableExecuteCommand` | boolean | Enable ECS Exec for service tasks. |
| `enableECSManagedTags` | boolean | Apply ECS managed tags to tasks. |
| `propagateTags` | string | Propagate `SERVICE` or `TASK_DEFINITION` tags. |
| `platformVersion` | string | Fargate platform version. |
| `healthCheckGracePeriodSeconds` | integer | Time to ignore load balancer health checks after task start. |
| `placementConstraints`, `placementStrategy` | list | ECS task placement constraints and strategies. |
| `tags` | map | Key-value pairs for resource tagging. |

`apply` reconciles the task definition, desired count, configured service settings, and tags. Fields left out of the file are not treated as requests to clear an existing setting. Auto scaling is applied through Application Auto Scaling; the caller needs permission to register scalable targets and scaling policies.

For existing clusters, `apply` reconciles configured capacity providers, the default capacity-provider strategy, Service Connect defaults, and tags. Providers named in a configured default strategy are attached automatically when `capacityProviders` is omitted. If you set `capacityProviders` without a strategy, keep every provider used by the cluster's existing default strategy in the list; apply rejects inconsistent combinations before updating AWS.

ECS does not support changing directly from one launch type to another through `UpdateService`. When `launchType` differs, apply returns an error instead of silently ignoring it; use a supported capacity-provider strategy migration or replace the service.

### ECS Express Mode Services

Express services use their own resource shape and can also be managed declaratively:

```bash
ecsctl apply -f express.yaml --wait
ecsctl express list --cluster production
```

See the [Express Mode guide](express.md) for the supported fields and lifecycle commands.

---

## The `create` Command

The `create` command explicitly creates new resources on AWS and registers them in the remote state backend (if configured). Unlike `apply`, it will fail if the resource already exists. 

You can run `create` in two ways: using a **YAML configuration file** or using **inline CLI flags**.

### Method A: Using a YAML Config File (-f)
This is the recommended method for tracking resources in git:

```bash
# Create a cluster from config
ecsctl create cluster -f cluster.yaml

# Create a service from config
ecsctl create service -f service.yaml
```

### Method B: Using Inline CLI Flags
If you want to quickly spin up a resource without writing a YAML file, pass the configuration as flags:

```bash
# Create a cluster using flags
ecsctl create cluster --name my-cluster --region us-east-1 --capacity-providers FARGATE,FARGATE_SPOT

# Create a service using flags
ecsctl create service --name my-service --cluster my-cluster --task-definition my-task:3 \
  --subnets subnet-abc123 --security-groups sg-abc123
```


---

## The `delete` Command

To delete resources defined in your files, or directly by their name:

```bash
# Delete a service by name
ecsctl delete service my-service --cluster my-cluster

# Delete a cluster by name
ecsctl delete cluster my-cluster
```
