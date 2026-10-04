# ecsctl

[![CI](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml/badge.svg)](https://github.com/Roslaan001/ecsctl/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/roslaan001/ecsctl)](https://goreportcard.com/report/github.com/roslaan001/ecsctl)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/Roslaan001/ecsctl/blob/main/LICENSE)

`ecsctl` is a command-line tool for creating and operating Amazon ECS clusters, services, and tasks. Use configuration files when you want to describe a resource and reconcile it over time. Use direct commands for one-time operations such as running a task, checking logs, or deploying a new image.

Use the guides in this order if you are new to ecsctl:

1. [Install ecsctl and configure AWS access](installation.md).
2. [Create or update clusters and services](declarative.md).
3. [Inspect resources and run common operations](inspection.md) and [commands](commands.md).
4. [Set up shared remote state](state.md) if you want ecsctl to track resources in S3.

## What you need

To manage live resources, you need:

- An AWS account with an ECS cluster or permission to create one.
- AWS credentials with permission to perform the operation you are running.
- An AWS Region. You can set it in your AWS profile or pass `--region` to a command.

You do not need to initialize remote state to use ecsctl. Without a configured state context, commands that support state tracking still call AWS directly. Remote state is an optional shared record of the resources you manage.

## Quick start

First, install ecsctl and configure AWS credentials. See the [installation guide](installation.md) for the supported credential methods. Confirm the CLI is available:

```bash
ecsctl version
```

Choose a name and Region for your cluster. Save this configuration as `cluster.yaml`:

```yaml
name: my-cluster
region: eu-west-2
capacityProviders:
  - FARGATE
  - FARGATE_SPOT
tags:
  environment: development
```

Apply the file to AWS:

```bash
ecsctl apply -f cluster.yaml
```

`apply` creates the cluster if it does not exist. If it already exists, ecsctl reconciles the fields in the file and leaves fields you did not specify unchanged. The YAML file is an input on your machine; ecsctl does not create it for you.

Confirm the cluster exists:

```bash
ecsctl describe cluster my-cluster
```

This creates a cluster only. To run an application, you also need an ECS task definition and a service configuration. Continue with the [declarative management guide](declarative.md), or use [ECS Express Mode](express.md) if you want ECS to manage the service's ingress and scaling resources.

## Optional: share resource state with a team

Remote state records which resources ecsctl manages in an S3 bucket. Set it up when you want multiple operators or automation to share that record:

```bash
ecsctl state init --context development --bucket my-ecsctl-state --region eu-west-2
```

This command creates the bucket if it does not exist, enables bucket versioning, and saves the `development` context in `~/.ecsctl/config.yaml`. Your AWS identity needs permission to inspect and create the bucket and enable versioning. See [Remote State](state.md) for contexts, imports, and recovery details.

## Guides

- [Installation and AWS credentials](installation.md)
- [Declarative configuration](declarative.md)
- [ECS Express Mode](express.md)
- [Operations and commands](commands.md)
- [Resource inspection](inspection.md)
- [Remote state](state.md)
- [Task definitions and one-off tasks](tasks.md)
- [Complete CLI reference](reference.md)
- [Release notes and upgrade guidance](releases.md)
- [Uninstallation](uninstallation.md)
