# ecsctl

[![CI](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml/badge.svg)](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/roslaan001/ecsctl)](https://goreportcard.com/report/github.com/roslaan001/ecsctl)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://github.com/Roslaan001/ecsctl/blob/main/LICENSE)

`ecsctl` is a CLI tool for managing Amazon ECS (Elastic Container Service) resources, heavily inspired by `eksctl` for EKS. It simplifies operating ECS clusters, services, and tasks by providing both a declarative YAML-based GitOps workflow and simple, fast imperative commands for day-to-day operations.


---

## Key Features

* 🚀 **Declarative Resource Management**: Create, update, or reconcile ECS clusters and services using GitOps-friendly YAML configurations (with support for `--dry-run` and `--wait`).
* ⚡ **Imperative Operations**: Stream logs across service tasks (`ecsctl logs`), open interactive shells (`ecsctl exec`), deploy images while preserving task-definition settings (`ecsctl deploy`), and run or stop standalone tasks.
* 🚀 **Express Mode**: Create, update, inspect, and delete ECS Express Mode services with managed ingress and scaling (`ecsctl express`).
* 📈 **Service Controls**: Reconcile capacity providers, deployment safety, load balancing, Service Connect, tags, placement, ECS Exec, and Application Auto Scaling from YAML.
* 📊 **Inspection & Querying**: Fast listing (`ecsctl list`) and detailed descriptions (`ecsctl describe`) of ECS clusters, services, and tasks.
* 🔒 **Remote State Backend**: Track your ecsctl-managed resources in an S3-based remote state context manager, similar to Terraform, supporting context switching (`ecsctl state`).
* 🔑 **Native AWS Auth**: Integrates natively with the standard AWS credential chain (`~/.aws/credentials`, IAM roles, environment variables).

---

## Quick Start

### 1. Install ecsctl
Run the installation script to fetch the latest precompiled release:

```bash
curl -fsSL https://raw.githubusercontent.com/Roslaan001/ecsctl/main/install.sh | sh
```

### 2. Configure AWS Credentials
Ensure you have your AWS credentials configured:

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"
```

### 3. Initialize Remote State
Start tracking your resources in a shared S3 state bucket:

```bash
ecsctl state init --context production --bucket my-ecsctl-production-state --region us-east-1
```

### 4. Create your First Cluster
Create a file named `cluster.yaml`:

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

Apply the configuration:

```bash
ecsctl apply -f cluster.yaml --wait
```

### 5. Inspect the Cluster
Retrieve the live configuration and status of the cluster:

```bash
ecsctl describe cluster my-cluster
```
