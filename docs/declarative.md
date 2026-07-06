# Declarative Resource Management

`ecsctl` supports managing Amazon ECS resources declaratively using YAML configuration files. This allows you to store your infrastructure configurations in git and build GitOps workflows.

> [!TIP]
> Ready-to-use templates are available in the [examples/](https://github.com/Roslaan001/ecsctl/tree/main/examples) directory of the repository.

---

## The `apply` Command

The `apply` command creates a resource if it doesn't exist, or reconciles its configurations if there are any differences (configuration drift) between your local YAML file and the live AWS resource state.

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
| `tags` | map | Key-value pairs for resource tagging. |

---

### 2. ECS Services (`service.yaml`)

Specify the configuration for an ECS service running inside a cluster.

```yaml
name: my-service
cluster: my-cluster
taskDefinition: my-task-def:3
launchType: FARGATE
desiredCount: 2
network:
  subnets:
    - subnet-0bb123456789abcde
    - subnet-0cc123456789abcde
  securityGroups:
    - sg-0aa123456789abcde
  assignPublicIp: ENABLED
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
| `desiredCount` | integer | Number of tasks to keep running. |
| `network` | object | Network configuration for the tasks. |
| `network.subnets` | list | Subnet IDs where tasks will be spawned. |
| `network.securityGroups`| list | Security Group IDs to associate with the tasks. |
| `network.assignPublicIp`| string | Whether to assign public IP (`ENABLED` or `DISABLED`). |
| `tags` | map | Key-value pairs for resource tagging. |

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
