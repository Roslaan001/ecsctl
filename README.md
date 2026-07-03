# ecsctl

[![CI](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml/badge.svg)](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/roslaan001/ecsctl)](https://goreportcard.com/report/github.com/roslaan001/ecsctl)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A CLI tool for managing Amazon ECS resources, inspired by `eksctl`.

## Installation

### Using `go install`

```bash
go install github.com/roslaan001/ecsctl@latest
```

### From source

```bash
git clone https://github.com/Roslaan001/ecsctl.git
cd ecsctl
make install
```

### Prerequisites

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

Explicitly create clusters or services from config files:

```bash
ecsctl create cluster -f cluster.yaml
ecsctl create service -f service.yaml
```

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

---

## Global Flags

The following flags can be passed to any subcommand to override default configurations:

| Flag        | Description                                                             |
|-------------|-------------------------------------------------------------------------|
| `--region`  | AWS region (overrides environment variables and configurations)         |
| `--profile` | AWS profile to authenticate with                                       |
| `--context` | Named state context to use (overrides active context in local config)   |

## Build & Test

```bash
make build     # builds binary to ./bin/ecsctl
make test      # runs unit tests
make lint      # runs golangci-lint checking
make clean     # cleans up build outputs and binaries
```
