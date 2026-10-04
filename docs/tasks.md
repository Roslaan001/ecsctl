# Task definitions and one-off tasks

An ECS task definition is a reusable blueprint for a task. It describes the container image, CPU and memory, roles, environment, logging, and other runtime settings. An ECS service keeps tasks running and replaces them when needed; a one-off task runs independently and exits when its command finishes.

This guide covers registering a task definition and starting or stopping one-off Fargate tasks. It does not cover creating an ECS service; see [Declarative Resource Management](declarative.md) for services.

## Register a task definition

`ecsctl register task-definition` reads an ECS task definition document in JSON format. The file must include a `family` and `containerDefinitions`:

```json
{
  "family": "batch-job",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "256",
  "memory": "512",
  "containerDefinitions": [
    {
      "name": "worker",
      "image": "public.ecr.aws/docker/library/busybox:latest",
      "essential": true,
      "command": ["sh", "-c", "echo job complete"]
    }
  ]
}
```

Save it as `task-definition.json`, then register it:

```bash
ecsctl register task-definition -f task-definition.json
```

ECS assigns a revision number to the task definition. Use the returned family and revision, such as `batch-job:1`, when starting a task. For all accepted fields, see the [Amazon ECS `RegisterTaskDefinition` API](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RegisterTaskDefinition.html).

`ecsctl deploy` uses the current task definition as a starting point, changes the selected container's image, registers a new revision, and updates the service. It preserves the other registrable task-level settings. See [Operations & Commands](commands.md).

`ecsctl describe service` shows each deployment's task definition, creation time,
and rollout reason. `ecsctl rollback SERVICE --cluster CLUSTER` updates an ECS
rolling service to the task definition from its previous completed deployment;
add `--wait` to wait for it to stabilize.

## Run a one-off ECS task

Use `run-task` for work that should run once without an ECS service keeping it alive. Unless you set `--launch-type`, ECS uses the cluster's default capacity-provider strategy. For Fargate tasks that use `awsvpc` networking, provide subnets and, when needed, security groups that allow the container to reach its dependencies:

```bash
ecsctl run-task \
  --cluster production \
  --task-definition batch-job:1 \
  --count 1 \
  --subnets subnet-a,subnet-b \
  --security-groups sg-a
```

The command starts one task and prints its identifier. Increase `--count` to start more than one copy. The task may stop on its own when its container process exits; ECS does not restart it through a service scheduler.

ecsctl leaves public IP assignment disabled by default. Set `--assign-public-ip ENABLED` only when that matches your VPC design. Otherwise, the task's subnets need a route or VPC endpoint for the services the container must reach.

## Stop a running task

Use the task ID or full task ARN printed by `run-task` or `ecsctl list tasks`:

```bash
ecsctl stop-task --cluster production --task <task-id-or-arn>
```

This requests that ECS stop the task; the task can remain in `STOPPING` briefly. Stopping it interrupts its current work. It does not delete the task definition or create a replacement task. Use `--reason` to include a reason in the stop request.

## Investigate a task that stopped or failed

Find stopped tasks, then inspect ECS stop details, per-container exit codes and errors, and network attachment information:

```bash
ecsctl list tasks --cluster production --desired-status STOPPED
ecsctl describe task <task-id-or-arn> --cluster production
```
