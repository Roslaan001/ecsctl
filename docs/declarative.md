# Declarative Resource Management

`ecsctl` can create or update ECS clusters and services from YAML configuration files. The file describes the settings you want ecsctl to manage, so you can review changes in Git and apply them again later.

The configuration file stays on your machine or in your repository. It is not remote state: S3 remote state is optional and is used to record which resources ecsctl has tracked. See [Remote State](state.md) for details.

> [!TIP]
> Ready-to-use templates are available in the [examples/](https://github.com/Roslaan001/ecsctl/tree/main/examples) directory of the repository.

---

## The `apply` Command

The `apply` command reads one configuration file and detects whether it describes a cluster, regular ECS service, or Express service. It creates the resource if it is missing. If it exists, ecsctl compares the fields in the file with AWS and updates configured fields that differ. Fields you leave out are not treated as requests to clear existing settings.

`apply` sends changes to AWS unless you include `--dry-run`. It does not require an S3 state context.

```bash
ecsctl apply -f <configuration-file>.yaml [flags]
```

### Preview changes
To see what ecsctl would change without sending the update to AWS, use `--dry-run`:

```bash
ecsctl apply -f cluster.yaml --dry-run
```

### Wait for a service
ECS may take time to start tasks and make a service healthy. Add `--wait` when you want the command to stay open until the service reaches a stable state:

```bash
ecsctl apply -f service.yaml --wait
```

For an existing resource, `--dry-run` lists the configured fields that differ from AWS. It does not include fields omitted from the YAML. For a service, omitting `desiredCount` leaves the live count alone; set `desiredCount: 0` to intentionally scale it to zero. For list-valued settings such as `capacityProviders`, `capacityProviderStrategy`, `loadBalancers`, `serviceRegistries`, `placementConstraints`, and `placementStrategy`, use an explicit empty list (`[]`) to clear existing values; omit the property to leave it unchanged.

For a cluster file, `region` selects the AWS region unless the global `--region` flag overrides it.

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

`apply` reconciles the task definition, desired count, configured service settings, and tags. For Fargate services, include the VPC subnets and security groups the tasks should use. Auto scaling is configured through Application Auto Scaling; the AWS identity running ecsctl needs permission to register scalable targets and scaling policies.

For existing clusters, `apply` reconciles configured capacity providers, the default capacity-provider strategy, Service Connect defaults, and tags. Providers named in a configured default strategy are attached automatically when `capacityProviders` is omitted. If you set `capacityProviders` without a strategy, keep every provider used by the cluster's existing default strategy in the list; apply rejects inconsistent combinations before updating AWS.

ECS does not support changing directly from one launch type to another through `UpdateService`. When `launchType` differs, apply returns an error instead of silently ignoring it; use a supported capacity-provider strategy migration or replace the service.

### ECS Express Mode services

Express services use a different configuration shape from regular ECS services. They can also be managed declaratively:

```bash
ecsctl apply -f express.yaml --wait
ecsctl express list --cluster production
```

`express.yaml` must contain Express service settings. It is not interchangeable with a regular `service.yaml`. See the [Express Mode guide](express.md) for a complete example, required fields, and the create/update workflow.

---

## The `create` Command

The `create` command explicitly creates a new cluster or service in AWS. Unlike `apply`, it fails if that resource already exists. If an S3 state context is configured, ecsctl also records the resource there; without one, the AWS resource is still created.

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
ecsctl create cluster --name my-cluster --region us-east-1
ecsctl create cluster --name my-ec2-cluster --region us-east-1 --ec2

# Add Fargate Spot explicitly
ecsctl create cluster --name my-cluster --region us-east-1 --capacity-providers FARGATE,FARGATE_SPOT

# Create a service using flags
ecsctl create service --name my-service --cluster my-cluster --task-definition my-task:3 \
  --subnets subnet-abc123 --security-groups sg-abc123
```

Without `--ec2`, `create cluster` attaches `FARGATE` and uses it as the default
capacity-provider strategy. With `--ec2`, ecsctl uses public subnets in the
region's default VPC, an ECS-optimized Amazon Linux 2023 image, a `t3.small`
instance, and an EC2 Auto Scaling capacity provider. It launches one instance by
default; `--ec2-instance-type` and `--ec2-count` override these defaults. The
EC2-backed cluster keeps Fargate attached as an option but defaults workloads to
EC2. Creating and running EC2 instances incurs AWS charges.

If you provide a custom Fargate provider list, ecsctl uses `FARGATE` as the
strategy only when it appears in that list; set
`defaultCapacityProviderStrategy` in YAML to choose another default.


---

## The `delete` command

Delete an existing resource by its name. These commands do not read a YAML file:

```bash
# Delete a service by name
ecsctl delete service my-service --cluster my-cluster

# Delete a cluster by name
ecsctl delete cluster my-cluster

# Preview a deletion without calling AWS
ecsctl delete service my-service --cluster my-cluster --dry-run

# Confirm an automated deletion explicitly
ecsctl delete cluster my-cluster --yes
```

Interactive deletion asks you to type the resource name. In non-interactive use, pass
`--yes` to confirm explicitly. Use `--dry-run` to preview the requested deletion
without calling AWS. Deleting a service drains and stops its running tasks. Deleting
a cluster requires it to contain no services or other resources that prevent
deletion; `--force` drains and deletes its services first.
