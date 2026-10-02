# CLI Reference

Use this page to find a command, its common syntax, and the flags it accepts. For step-by-step examples, see the [guides](index.md).

## Commands at a glance

| Command | Purpose |
|---|---|
| `apply` | Create or update resources from YAML. |
| `create` | Create a cluster or service. |
| `delete` | Delete a cluster or service. |
| `describe` | Show a cluster or service. |
| `list` | List clusters, services, or tasks. |
| `deploy` | Roll out a new service image. |
| `scale` | Change a service's desired task count. |
| `logs` | Read or follow service logs. |
| `exec` | Open a shell or run a command in a task. |
| `express` | Manage ECS Express Mode services. |
| `register task-definition` | Register a task definition from JSON. |
| `run-task` / `stop-task` | Start or stop standalone tasks. |
| `state` | Manage remote state and tracked resources. |
| `version` | Print build information. |
| `completion` | Generate shell completion scripts. |
| `help` | Show help for a command. |

## Global flags

These flags can be used with any command or subcommand.

| Flag | Type | Description |
|---|---|---|
| `--region` | string | AWS region to target, such as `us-east-1`. Overrides environment settings. |
| `--profile` | string | AWS profile to use from `~/.aws/credentials`. |
| `--context` | string | Remote state context to use for this command. |
| `-h, --help` | boolean | Show help for the current command. |

## Resource commands

### `apply`

Create a cluster, ECS service, or Express service from YAML, or update configured fields on an existing resource.

```text
ecsctl apply -f FILE [--dry-run] [--wait]
```

| Flag | Type | Description |
|---|---|---|
| `-f, --file` | string | YAML configuration file. Required. |
| `--dry-run` | boolean | Show the planned changes without applying them. |
| `--wait` | boolean | Wait for the resource to reach a stable state. |

### `create`

Create a cluster or regular ECS service. Express services are created with `express create`.

#### `create cluster`

```text
ecsctl create cluster -f FILE
ecsctl create cluster --name NAME [--region REGION] [--capacity-providers LIST]
```

| Flag | Type | Description |
|---|---|---|
| `-f, --file` | string | YAML configuration file. |
| `--name` | string | Cluster name. Required when `-f` is omitted. |
| `--region` | string | AWS region. |
| `--capacity-providers` | string slice | Capacity providers, such as `FARGATE,FARGATE_SPOT`. Defaults to `FARGATE`; incompatible with `--ec2`. |
| `--ec2` | boolean | Create an EC2-backed cluster in the default VPC. |
| `--ec2-instance-type` | string | EC2 instance type for `--ec2`. Defaults to `t3.small`. |
| `--ec2-count` | int | Initial EC2 instance count for `--ec2`. Defaults to `1`. |
| `--tags` | string array | Tags in `key=value` form. |

Without `--ec2`, new clusters default to Fargate. EC2-backed clusters default to EC2 capacity; creating and running instances incurs AWS charges.

#### `create service`

```text
ecsctl create service -f FILE
ecsctl create service --name NAME --cluster CLUSTER --task-definition FAMILY:REV
```

| Flag | Type | Description |
|---|---|---|
| `-f, --file` | string | YAML configuration file. |
| `--name` | string | Service name. Required when `-f` is omitted. |
| `--cluster` | string | Cluster name. Required when `-f` is omitted. |
| `--task-definition` | string | Task definition family and revision. Required when `-f` is omitted. |
| `--launch-type` | string | `FARGATE` or `EC2`. Defaults to the cluster's capacity-provider strategy. |
| `--scheduling-strategy` | string | `REPLICA` or `DAEMON`. Defaults to `REPLICA`. |
| `--desired-count` | int | Number of tasks to run. Defaults to `1`. |
| `--subnets` | string slice | Subnet IDs. |
| `--security-groups` | string slice | Security group IDs. |
| `--assign-public-ip` | string | `ENABLED` or `DISABLED`. Defaults to `ENABLED`. |
| `--tags` | string array | Tags in `key=value` form. |
| `--wait` | boolean | Wait for the service to reach steady state. |

### `delete`

Delete a resource by name.

| Syntax | Flags |
|---|---|
| `ecsctl delete cluster NAME` | Optional `--force` to delete its services first. |
| `ecsctl delete service NAME` | Required `--cluster CLUSTER`. |

### `describe`

Show live configuration and runtime status.

| Syntax | Required flags |
|---|---|
| `ecsctl describe cluster NAME` | — |
| `ecsctl describe service NAME` | `--cluster CLUSTER` |

### `list`

List resources from saved state or query live AWS resources with `--live`.

| Syntax | Flags |
|---|---|
| `ecsctl list clusters` | `--live` |
| `ecsctl list services` | `--cluster CLUSTER`, `--live` |
| `ecsctl list tasks` | `--cluster CLUSTER`, optional `--service NAME`, `--live` |

### `deploy`

Roll out a new image for a service.

```text
ecsctl deploy SERVICE --cluster CLUSTER --image IMAGE [--container NAME]
```

| Flag | Type | Description |
|---|---|---|
| `--cluster` | string | Target cluster. Required. |
| `--image` | string | New image and tag, such as `nginx:latest`. Required. |
| `--container` | string | Container to update. Defaults to the first container. |
| `--wait` | boolean | Wait for the deployment to complete. |

### `scale`

Set the desired task count for a service.

```text
ecsctl scale SERVICE --cluster CLUSTER --desired COUNT
```

| Flag | Type | Description |
|---|---|---|
| `--cluster` | string | Target cluster. Required. |
| `--desired` | integer | Desired number of tasks. Required. |

## Task operations

### `logs`

Read or follow logs from a service's tasks.

```text
ecsctl logs SERVICE --cluster CLUSTER [--follow] [--tail LINES]
```

| Flag | Type | Description |
|---|---|---|
| `--cluster` | string | Target cluster. Required. |
| `-f, --follow` | boolean | Stream new log lines. |
| `--tail` | integer | Number of recent lines to show. Defaults to `50`. |
| `--container` | string | Container name. Defaults to the first container. |

### `exec`

Open a shell or run a command in a task for a service.

```text
ecsctl exec SERVICE --cluster CLUSTER [--container NAME] [--command COMMAND] [--task TASK]
```

| Flag | Type | Description |
|---|---|---|
| `--cluster` | string | Target cluster. Required. |
| `--container` | string | Container name. |
| `--command` | string | Command to run. Defaults to a shell. |
| `--task` | string | Specific task to use. Otherwise, ecsctl selects a running task. |

### Standalone tasks

| Command | Syntax | Description |
|---|---|---|
| `register task-definition` | `ecsctl register task-definition -f FILE` | Register a task definition JSON document. |
| `run-task` | `ecsctl run-task --cluster CLUSTER --task-definition FAMILY:REV` | Start standalone tasks. See its flags below. |
| `stop-task` | `ecsctl stop-task --cluster CLUSTER --task TASK_ID_OR_ARN` | Stop a running task. Supports `--reason`. |

#### `run-task` flags

| Flag | Type | Description |
|---|---|---|
| `--cluster` | string | Target cluster. Required. |
| `--task-definition` | string | Task definition family and revision, or ARN. Required. |
| `--count` | int | Number of tasks to start. Defaults to `1`. |
| `--launch-type` | string | `FARGATE` or `EC2`. Defaults to the cluster's capacity-provider strategy. |
| `--subnets` | string slice | Subnet IDs. |
| `--security-groups` | string slice | Security group IDs. |
| `--assign-public-ip` | string | `ENABLED` or `DISABLED`. Defaults to `DISABLED`. |

#### `stop-task` flags

| Flag | Type | Description |
|---|---|---|
| `--cluster` | string | Target cluster. Required. |
| `--task` | string | Task ID or ARN. Required. |
| `--reason` | string | Stop reason. Defaults to `Stopped by ecsctl`. |

## ECS Express Mode

Manage ECS Express Mode services. See the [Express Mode guide](express.md) for details.

| Subcommand | Syntax | Description |
|---|---|---|
| `create` | `ecsctl express create -f FILE [--wait]` | Create from YAML. `-f` is required; `--wait` defaults to `true`. Include `serviceName`, `infrastructureRoleArn`, and either `image` or `taskDefinitionArn`. |
| `update` | `ecsctl express update --service-arn ARN -f FILE [--wait]` | Update from YAML. `--service-arn` and `-f` are required; `--wait` defaults to `true`. |
| `describe` | `ecsctl express describe --service-arn ARN` | Show service status. `--service-arn` is required. |
| `delete` | `ecsctl express delete --service-arn ARN` | Begin managed service deletion. `--service-arn` is required. |
| `list` | `ecsctl express list [--cluster NAME]` | List Express Mode services. `--cluster` defaults to the default cluster. |

## Remote state

Use `state` to manage remote state contexts and tracked resources.

| Subcommand | Syntax | Description |
|---|---|---|
| `init` | `ecsctl state init --context NAME --bucket BUCKET --region REGION` | Initialize a context. `--bucket` and `--region` are required. `--context` defaults to `default`; `--key` defaults to the context name. Optional flags: `--profile`, `--kms-key-id`. |
| `list-contexts` | `ecsctl state list-contexts` | List configured contexts. |
| `use-context` | `ecsctl state use-context NAME` | Switch the active context. |
| `import cluster` | `ecsctl state import cluster NAME` | Import a cluster. Supports `--context` to select a context. |
| `import service` | `ecsctl state import service NAME --cluster CLUSTER` | Import a service. `--cluster` is required; supports `--context`. |
| `import express` | `ecsctl state import express NAME_OR_ARN [--cluster CLUSTER]` | Import an Express service and capture its configuration. Supports `--context`. |
| `config` | `ecsctl state config RESOURCE NAME [--cluster CLUSTER]` | Print captured YAML. Use `--cluster` to select a service in a cluster; an active context is required. |
| `show` | `ecsctl state show` | Show the active context and tracked resources. |
| `drift` | `ecsctl state drift` | Compare tracked configuration with live ECS resources. |
| `unlock` | `ecsctl state unlock --force` | Remove a stale lock after confirming no operation is active. |

## Shell completion

Generate a completion script for the shell you use:

```text
ecsctl completion bash
ecsctl completion zsh
ecsctl completion fish
ecsctl completion powershell
```

For help with a specific command or its flags, run:

```text
ecsctl --help
ecsctl create service --help
ecsctl help state import
```

## `version`

Print the ecsctl version, commit, and build information.
