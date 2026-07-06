# ecsctl

`ecsctl` is a CLI tool for managing Amazon ECS (Elastic Container Service) resources, heavily inspired by `eksctl` for EKS. It simplifies operating ECS clusters, services, and tasks by providing both a declarative YAML-based GitOps workflow and simple, fast imperative commands for day-to-day operations.

---

## Key Features

* 🚀 **Declarative Resource Management**: Create, update, or reconcile ECS clusters and services using GitOps-friendly YAML configurations (with support for `--dry-run` and `--wait`).
* ⚡ **Imperative Operations**: Stream logs in real-time (`ecsctl logs`), open interactive shells inside containers (`ecsctl exec`), deploy new container images (`ecsctl deploy`), and scale services (`ecsctl scale`).
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
