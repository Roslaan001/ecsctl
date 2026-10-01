# ecsctl

[![CI](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml/badge.svg)](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/roslaan001/ecsctl)](https://goreportcard.com/report/github.com/roslaan001/ecsctl)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A CLI tool for managing Amazon ECS resources, inspired by `eksctl`.

📖 **[Read the full documentation](https://roslaan001.github.io/ecsctl/)**

---

## Installation

### 1. Using the installation script (Recommended)

**macOS / Linux**:
```bash
curl -fsSL https://roslaan001.github.io/ecsctl/install.sh | sh
```

**Windows (PowerShell)**:
```powershell
irm https://roslaan001.github.io/ecsctl/install.ps1 | iex
```

### 2. Using `go install`

```bash
go install github.com/roslaan001/ecsctl@latest
```

### 3. From source

```bash
git clone https://github.com/Roslaan001/ecsctl.git
cd ecsctl
make install
```

---

## Documentation

The full documentation is built with MkDocs and hosted on [GitHub Pages](https://roslaan001.github.io/ecsctl/). Pull requests build the site, and pushes to `main` or `master` publish it to the `gh-pages` branch through [the docs workflow](.github/workflows/docs.yml). Installers download release archives directly from GitHub Releases.

To preview the documentation locally, run:

```bash
make docs-serve
```

This will automatically create a local Python virtual environment, install the `mkdocs-material` theme, and start a live-reloading server at `http://127.0.0.1:8000`.

---

- Go 1.26+
- AWS credentials configured (`~/.aws/credentials`, environment variables, or IAM role)
- [Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) (required for `ecsctl exec`)

## Authentication

`ecsctl` uses the standard AWS credential chain — environment variables, `~/.aws/credentials`, or IAM roles. Use `--region` and `--profile` flags to override per command.

---

## Declarative Resource Management

You can manage resources declaratively using YAML configurations.

### 1. Apply (Create or Update)

The `apply` command creates resources if they are absent or reconciles them if there is a configuration drift.

```bash
# Preview changes without applying
ecsctl apply -f cluster.yaml --dry-run

# Apply configuration and wait for stability
ecsctl apply -f service.yaml --wait
```

**cluster.yaml**
```yaml
name: my-cluster
region: us-east-1
capacityProviders:
  - FARGATE
  - FARGATE_SPOT
tags:
  env: production
  team: platform
```

**service.yaml**
```yaml
name: my-service
cluster: my-cluster
taskDefinition: my-task:3
launchType: FARGATE
desiredCount: 2
network:
  subnets:
    - subnet-abc123
  securityGroups:
    - sg-abc123
  assignPublicIp: ENABLED
tags:
  env: production
```

### 2. Create

Explicitly create clusters or services using config files or inline flags:

```bash
# Create from configuration files
ecsctl create cluster -f cluster.yaml
ecsctl create service -f service.yaml

# Create directly using inline flags
ecsctl create cluster --name my-cluster --region us-east-1
ecsctl create service --name my-service --cluster my-cluster --task-definition my-task:3
```

ECS service YAML can also configure capacity-provider strategies, deployment controllers and safety settings, load balancers, Service Connect, placement, ECS Exec, tags, and Application Auto Scaling. See [the declarative configuration guide](docs/declarative.md).

### ECS Express Mode

Create an Express Mode service, which lets ECS provision and manage its ingress, load balancer, scaling, logging, and alarms:

```bash
ecsctl express create -f express.yaml --wait
ecsctl express describe --service-arn "$SERVICE_ARN"
```

See the [Express Mode guide](docs/express.md) for the role requirements and YAML fields.

### Task definitions and one-off tasks

Register a task definition JSON file and run or stop standalone Fargate tasks:

```bash
ecsctl register task-definition -f task-definition.json
ecsctl run-task --cluster my-cluster --task-definition batch:1 --subnets subnet-a --security-groups sg-a
ecsctl stop-task --cluster my-cluster --task <task-id-or-arn>
```

See [the task guide](docs/tasks.md) for details.

### 3. Delete

Delete active clusters or services:

```bash
# Delete a service
ecsctl delete service my-service --cluster my-cluster

# Delete a cluster
ecsctl delete cluster my-cluster
```

---

## Imperative Commands & Operations

### Deploy

Update an ECS service with a new container image:

```bash
# Deploy to the default container
ecsctl deploy my-service --cluster my-cluster --image nginx:1.25

# Deploy to a specific container
ecsctl deploy my-service --cluster my-cluster --image nginx:1.25 --container web
```

### Scale

Scale the desired task count of an ECS service:

```bash
ecsctl scale my-service --cluster my-cluster --desired 3
```

### Logs

Stream or retrieve logs from a service's tasks:

```bash
# Fetch recent logs
ecsctl logs my-service --cluster my-cluster

# Tail logs with custom line count
ecsctl logs my-service --cluster my-cluster --tail 100

# Stream logs in real-time
ecsctl logs my-service --cluster my-cluster --follow
```

### Exec

Open an interactive shell or run a command inside a running service task container (requires ECS Exec enablement):

```bash
# Open interactive shell in default container
ecsctl exec my-service --cluster my-cluster

# Run in a specific container
ecsctl exec my-service --cluster my-cluster --container web

# Run a custom command
ecsctl exec my-service --cluster my-cluster --command "/bin/sh"
```

---

## Inspection & Querying

### List

List resources locally from remote state, or directly from live AWS if no state is configured:

```bash
# List all ECS clusters
ecsctl list clusters

# List all services in a cluster
ecsctl list services --cluster my-cluster

# List running tasks in a cluster or service
ecsctl list tasks --cluster my-cluster
ecsctl list tasks --cluster my-cluster --service my-service
```

### Describe

Retrieve detailed configuration and status information for a specific resource:

```bash
# Describe cluster configuration and stats
ecsctl describe cluster my-cluster

# Describe service configuration, deployment, and task status
ecsctl describe service my-service --cluster my-cluster
```

---

## Remote State Management

`ecsctl` supports storing resource tracking configurations remotely in an S3 bucket. This acts as a shared state file and context manager similar to Terraform state.

### 1. Initialize State Context

Creates an S3 bucket (or checks existence) in a given region, enables S3 versioning, and saves context details under `~/.ecsctl/config.yaml`.

```bash
ecsctl state init --context staging --bucket my-ecsctl-staging-state --region eu-west-1
ecsctl state init --context prod --bucket my-ecsctl-prod-state --region us-east-1 --profile prod-profile
```

### 2. Switch Contexts

```bash
# List configured contexts (* indicates current context)
ecsctl state list-contexts

# Switch the current active context
ecsctl state use-context staging
```

### 3. Import Existing Resources

Import your pre-existing AWS ECS resources into `ecsctl` state control:

```bash
# Import an existing cluster
ecsctl state import cluster my-existing-cluster --region us-east-1

# Import an existing service
ecsctl state import service my-existing-service --cluster my-existing-cluster --region us-east-1
```

### 4. Show Remote State

Print the S3 path and all tracked clusters and services inside the current state context:

```bash
ecsctl state show
```

### 5. Check for Drift

Compare saved configurations in the active state context with live ECS resources without changing them:

```bash
ecsctl state drift
```

The scan reports drift, resources that are up to date, skipped resources without a saved configuration, and errors. Use `--context` to scan a different state context.

### 6. State Locking

When a state context is active, resource-changing commands hold its S3 lock until the AWS operation and state update finish. If a command leaves a stale lock, confirm that no operation is still running before removing it:

```bash
ecsctl state unlock --force
```

See the [Remote State guide](docs/state.md) for context setup, imports, drift checks, and lock recovery.

---

## Global Flags

The following flags can be passed to any subcommand to override default configurations:

| Flag        | Description                                                             |
|-------------|-------------------------------------------------------------------------|
| `--region`  | AWS region (overrides environment variables and configurations)         |
| `--profile` | AWS profile to authenticate with                                       |
| `--context` | Named state context to use (overrides active context in local config)   |

## Build & Test

`make test-floci` uses dummy `test` credentials and a local Floci endpoint; it does not need an AWS account or real AWS keys. The current Floci ECS API does not list the Express Gateway operations, so Floci covers the standard ECS service lifecycle. A manually dispatched [AWS Express smoke workflow](.github/workflows/express-aws-smoke.yml) creates, waits for, describes, and deletes a one-task service. It requires the `AWS_SMOKE_ROLE_ARN` GitHub secret configured for OIDC, plus an existing test cluster and ECS infrastructure/execution roles. See the [Floci ECS support matrix](https://floci.io/floci/services/ecs/).

```bash
make build     # builds binary to ./bin/ecsctl
make test      # runs unit tests
make test-floci # runs ECS integration test against local Floci (requires Docker Compose or Podman)
make lint      # runs golangci-lint checking
make clean     # cleans up build outputs and binaries
```

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidance, commit message rules, pull request review, and CI checks.
