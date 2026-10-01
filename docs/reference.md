# CLI Reference

This page describes all commands, subcommands, and global flags available in `ecsctl`.

---

## Global Flags

The following flags can be supplied to **any** command or subcommand to override configured context or credentials on the fly.

| Flag | Shorthand | Type | Description |
|---|---|---|---|
| `--region` | - | string | AWS region to target (e.g. `us-east-1`). Overrides environment variables. |
| `--profile` | - | string | AWS profile name to authenticate with from `~/.aws/credentials`. |
| `--context` | - | string | Named remote state context to run this command under (temporary override). |

---

## Command Hierarchy

Below is the complete command tree for `ecsctl`.

```text
ecsctl
├── apply            - Create or update resources from a YAML config file
├── create           - Explicitly create resources (cluster, service) from YAML
│   ├── cluster
│   └── service
├── delete           - Delete resources (cluster, service) by name
│   ├── cluster
│   └── service
├── describe         - Show detailed live configuration and stats of a resource
│   ├── cluster
│   └── service
├── list             - List active resources from state (or live AWS)
│   ├── clusters
│   ├── services
│   └── tasks
├── deploy           - Update a service container image tag (rolling deploy)
├── scale            - Scale desired task count of a service
├── logs             - Aggregate and follow service task logs
├── exec             - Open interactive shell or run commands in container task
├── express          - Create, update, inspect, and delete ECS Express Mode services
├── register         - Register a task definition from ECS JSON
├── run-task         - Run one-off ECS tasks
├── stop-task        - Stop a running task
├── state            - Manage remote state backend contexts
│   ├── init
│   ├── list-contexts
│   ├── use-context
│   ├── import
│   ├── config
│   ├── show
│   ├── drift
│   └── unlock --force
└── version          - Print build version and compilation information
```

---

## Detailed Command Specifications

### `apply`
Create a cluster, regular ECS service, or Express service from a YAML configuration file, or update configured fields on an existing resource.
* **Flags**:
  * `-f, --file` (string, required): Path to YAML config file.
  * `--dry-run` (boolean): Show what changes would occur without applying them.
  * `--wait` (boolean): Wait for resource to reach a stable state before exiting.

### `create`
Create a new ECS resource and fail if it already exists. Clusters and regular ECS services can use a YAML file or inline flags. Express services require a YAML file.
* **Subcommands**:
  * `create cluster`: Create an ECS cluster.
    * **Flags**:
      * `-f, --file` (string): Path to YAML config.
      * `--name` (string): Cluster name (required if `-f` is omitted).
      * `--region` (string): AWS region.
      * `--capacity-providers` (string slice): Capacity providers, e.g., `FARGATE,FARGATE_SPOT`.
      * `--ec2` (boolean): Create an EC2-backed cluster in the default VPC.
      * `--ec2-instance-type` (string): EC2 instance type used with `--ec2` (defaults to `t3.small`).
      * `--ec2-count` (int): Initial EC2 instance count used with `--ec2` (defaults to 1).
      * `--tags` (string array): Tags as `key=value` pairs.
  * `create service`: Create an ECS service.
    * **Flags**:
      * `-f, --file` (string): Path to YAML config.
      * `--name` (string): Service name (required if `-f` is omitted).
      * `--cluster` (string): Cluster name (required if `-f` is omitted).
      * `--task-definition` (string): Task definition family:revision (required if `-f` is omitted).
      * `--launch-type` (string): `FARGATE` or `EC2` (defaults to the cluster capacity-provider strategy).
      * `--desired-count` (int): Number of tasks to run (defaults to 1).
      * `--subnets` (string slice): Subnet IDs.
      * `--security-groups` (string slice): Security Group IDs.
      * `--assign-public-ip` (string): `ENABLED` or `DISABLED` (defaults to `ENABLED`).
      * `--tags` (string array): Tags as `key=value` pairs.
      * `--wait` (boolean): Wait for the service to reach steady state.

### `delete`
Delete resources.
* **Subcommands**:
  * `delete cluster <name>`: Delete a cluster.
  * `delete service <name>`: Delete a service.
    * **Flags**:
      * `--cluster` (string, required): Cluster name.

### `describe`
Show detailed configuration and runtime status.
* **Subcommands**:
  * `describe cluster <name>`: Describe a cluster.
  * `describe service <name>`: Describe a service.
    * **Flags**:
      * `--cluster` (string, required): Cluster name.

### `list`
List resources from remote state or live AWS.
* **Flags**:
  * `--live`: Query AWS directly instead of using the saved state inventory.
* **Subcommands**:
  * `list clusters`: List clusters.
  * `list services`: List services in a cluster.
    * **Flags**:
      * `--cluster` (string, required): Cluster name.
  * `list tasks`: List running tasks.
    * **Flags**:
      * `--cluster` (string, required): Cluster name.
      * `--service` (string): Filter tasks by service name.

### `deploy`
Update a service container image tag (rolling deploy).
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `--image` (string, required): New container image tag (e.g., `nginx:latest`).
  * `--container` (string): Target container name in the task definition (defaults to first container).

### `scale`
Scale desired task count of a service.
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `--desired` (integer, required): Desired number of tasks to run.

### `logs`
Aggregate and stream logs from service tasks.
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `-f, --follow` (boolean): Stream logs in real-time.
  * `--tail` (integer): Number of recent lines to show (defaults to 50).

### `exec`
Open interactive shell or run commands in a container task.
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `--container` (string): Target container name.
  * `--command` (string): Command to execute (defaults to shell).
  * `--task` (string): Choose a specific task; otherwise ecsctl selects a running task for the service.

### `express`
Manage ECS Express Mode web services; see [Express Mode guide](express.md).
* `express create -f FILE [--wait]`: Create from a YAML file. It must include `serviceName`, `infrastructureRoleArn`, and either `image` or `taskDefinitionArn`.
* `express update --service-arn ARN -f FILE [--wait]`: Update from a YAML file.
* `express describe --service-arn ARN`: Show service status.
* `express delete --service-arn ARN`: Begin managed service deletion.
* `express list [--cluster NAME]`: List Express Mode services.

### Task definitions and one-off tasks
* `register task-definition -f FILE`: Register an ECS task definition JSON document.
* `run-task --cluster CLUSTER --task-definition FAMILY:REV`: Start standalone ECS tasks; supports `--count`, `--launch-type`, `--subnets`, `--security-groups`, and `--assign-public-ip`. By default, ECS uses the cluster's capacity-provider strategy.
* `stop-task --cluster CLUSTER --task TASK_ID_OR_ARN`: Stop a running task.

### `state`
Manage remote state backend contexts.
* **Subcommands**:
  * `state init`: Initialize state context.
    * **Flags**:
      * `--context` (string, required): Context name.
      * `--bucket` (string, required): S3 bucket name.
      * `--region` (string, required): AWS region.
      * `--profile` (string): AWS profile.
      * `--kms-key-id` (string): KMS Key ID for encryption.
  * `state list-contexts`: List configured contexts.
  * `state use-context <name>`: Switch active context.
  * `state import cluster <name>`: Import cluster to state.
  * `state import service <name>`: Import service to state.
    * **Flags**:
      * `--cluster` (string, required): Cluster name.
  * `state import express <name-or-arn>`: Import an Express service and capture its configuration.
    * **Flags**:
      * `--cluster` (string, required): Cluster name.
  * `state config <cluster|service|express> <name>`: Print captured YAML configuration. Service and Express resources accept `--cluster`; an active state context is required.
  * `state show`: Print details of active context and tracked resources.
  * `state drift`: Read-only comparison of tracked configurations with live ECS resources.
  * `state unlock --force`: Remove a stale state lock after confirming no operation is active.

### `version`
Print build version and compilation details.
