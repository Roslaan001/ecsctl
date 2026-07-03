# ecsctl

A CLI tool for managing Amazon ECS resources, inspired by `eksctl`.

## Installation

```bash
git clone https://github.com/roslaan001/ecsctl.git
cd ecsctl
make install
```

## Authentication

`ecsctl` uses the standard AWS credential chain — environment variables, `~/.aws/credentials`, or IAM roles. Use `--region` and `--profile` flags to override per command.

## Usage

### Clusters

```bash
# Create a cluster from a YAML config
ecsctl create cluster -f cluster.yaml

# Delete a cluster
ecsctl delete cluster my-cluster
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

### Services

```bash
# Create a service from a YAML config
ecsctl create service -f service.yaml

# Delete a service
ecsctl delete service my-service --cluster my-cluster

# Scale a service
ecsctl scale my-service --cluster my-cluster --desired 3
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

### Deploy

Update a service with a new Docker image:

```bash
ecsctl deploy my-service --cluster my-cluster --image nginx:1.25
ecsctl deploy my-service --cluster my-cluster --image nginx:1.25 --container web
```

### Logs

```bash
ecsctl logs my-service --cluster my-cluster
ecsctl logs my-service --cluster my-cluster --tail 100
ecsctl logs my-service --cluster my-cluster --follow
```

### Exec

Open a shell in a running task (requires ECS Exec enabled on the service):

```bash
ecsctl exec my-service --cluster my-cluster
ecsctl exec my-service --cluster my-cluster --container web
ecsctl exec my-service --cluster my-cluster --command "/bin/bash"
```

## Global Flags

| Flag        | Description                          |
|-------------|--------------------------------------|
| `--region`  | AWS region (overrides env/config)    |
| `--profile` | AWS profile to use                   |

## Build

```bash
make build     # builds to ./bin/ecsctl
make test      # runs tests
make lint      # runs golangci-lint
make clean     # removes build artifacts
```
