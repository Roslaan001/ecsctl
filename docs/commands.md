# Operations & Imperative Commands

While declarative configuration is preferred for managing resources, `ecsctl` provides powerful imperative commands for daily maintenance, debugging, and rapid deployments.

---

## 🚢 Deploying Images (`deploy`)

Update an existing ECS service with a new container image tag. This command triggers a rolling deployment by updating the service's task definition with the new image.

```bash
ecsctl deploy <service-name> --cluster <cluster-name> --image <image-url:tag> [flags]
```

### Basic Deploy
Deploy a new image to the default container in the task definition:

```bash
ecsctl deploy my-service --cluster my-cluster --image nginx:1.25.1
```

### Specify a Container Name
If your task definition contains multiple containers, specify the target container using the `--container` flag:

```bash
ecsctl deploy my-service --cluster my-cluster --image redis:7.2-alpine --container cache
```

## ↩️ Rolling Back a Deployment (`rollback`)

Restore the task definition used by the most recent completed deployment before
the service's current task definition:

```bash
ecsctl rollback my-service --cluster my-cluster --wait
```

Use `ecsctl describe service my-service --cluster my-cluster` to review task
definition revisions, deployment timestamps, and rollout reasons. Rollback
supports ECS rolling deployments. Services managed by CodeDeploy or an external
deployment controller must be rolled back through that controller.

---

## 📈 Scaling Services (`scale`)

Quickly scale the running task count of an ECS service up or down.

```bash
ecsctl scale <service-name> --cluster <cluster-name> --desired <count> [flags]
```

### Example
Scale the desired task count to 5:

```bash
ecsctl scale my-service --cluster my-cluster --desired 5
```

This changes the service's desired task count. If Application Auto Scaling is also configured, its policies can change the count again as load changes.

---

## 📋 Fetching Logs (`logs`)

Stream or fetch CloudWatch Logs events from the containers in an ECS service. The task definition must send container logs to CloudWatch Logs, and your AWS identity must be allowed to read those log events. ecsctl gathers matching logs across tasks in the service.

```bash
ecsctl logs <service-name> --cluster <cluster-name> [flags]
```

### Tail Logs
Fetch the last 100 log lines:

```bash
ecsctl logs my-service --cluster my-cluster --tail 100
```

### Follow Logs
Stream logs in real-time to your terminal (similar to `tail -f`):

```bash
ecsctl logs my-service --cluster my-cluster --follow
```

---

## 💻 Interactive Container Shell (`exec`)

Open an interactive bash/sh session or execute a single command inside a running task container using ECS Exec.

> [!IMPORTANT]
> To use this command, install the AWS Session Manager Plugin locally and make sure ECS Exec is enabled for the task. Your AWS identity and the task's IAM role also need the permissions required by ECS Exec. See [AWS ECS Exec prerequisites](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html).

```bash
ecsctl exec <service-name> --cluster <cluster-name> [flags]
```

### Interactive Shell
Open a shell session (defaults to `/bin/sh` or `/bin/bash` in the default container):

```bash
ecsctl exec my-service --cluster my-cluster
```

### Specify Container and Command
Specify a particular container and command to run:

```bash
ecsctl exec my-service --cluster my-cluster --container web-app --command "python manage.py migrate"
```

---

## 🔌 Port Forwarding (`port-forward`)

Forward a local port to a container port in a running ECS task using AWS Systems Manager Session Manager. This allows you to securely access databases, APIs, or internal admin dashboards running inside private containers directly from your workstation without public IPs, open inbound security groups, or bastion hosts.

> [!IMPORTANT]
> Requires the AWS Session Manager Plugin installed locally, and ECS Exec enabled for the target task/service.

```bash
ecsctl port-forward <service-name> <local-port>:<container-port> --cluster <cluster-name> [flags]
```

### Forward to a Service
Forward local port 8080 to container port 80 on a running task:

```bash
ecsctl port-forward my-service 8080:80 --cluster my-cluster
```

If local and container ports are the same:

```bash
ecsctl port-forward my-service 8080 --cluster my-cluster
```

### Specify a Container Name
When a task definition contains multiple containers, select the target container with `--container`:

```bash
ecsctl port-forward my-service 5432:5432 --cluster my-cluster --container postgres
```

### Forward Directly to a Specific Task
Forward traffic directly to an individual task ID without a service:

```bash
ecsctl port-forward 8080:80 --cluster my-cluster --task abc123def456
```

