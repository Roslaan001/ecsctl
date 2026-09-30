# ECS Express Mode

ECS Express Mode creates a service and lets Amazon ECS manage supporting resources such as ingress, load balancing, scaling, logging, and alarms. Use it when you want ECS to operate those pieces for you. For a standard ECS service whose networking and load balancer you configure yourself, see [Declarative Resource Management](declarative.md).

## Before you create a service

You need AWS credentials, a Region, a target ECS cluster (or the AWS default cluster), and an infrastructure role ARN that ECS can use to provision and manage the service's supporting resources. Your AWS identity must be allowed to create the Express service. The infrastructure role is required in the configuration.

You must also choose how ECS gets the application container:

- Set `image` to a container image URI. You can also set container options such as port, environment variables, secrets, and CPU and memory.
- Or set `taskDefinitionArn` to an existing task definition ARN. When you use this option, do not also set `image` or container fields: `executionRoleArn`, `taskRoleArn`, `cpu`, `memory`, `cpuArchitecture`, `command`, `containerPort`, `environment`, `secrets`, `awsLogsConfiguration`, or `repositoryCredentials`.

## Create a configuration file

`express create`, `express update`, and `apply` read their settings from a file. The file must contain YAML; `.yaml` is the conventional extension, but ecsctl reads the file contents rather than checking its name. There is no inline-flags form of `express create` or `express update`.

Save a file named `express.yaml` and replace the example names, ARNs, image URI, network IDs, and values with those for your AWS account:

```yaml
serviceName: orders-api
cluster: production
infrastructureRoleArn: arn:aws:iam::123456789012:role/ECSExpressInfrastructureRole
executionRoleArn: arn:aws:iam::123456789012:role/ecsTaskExecutionRole
taskRoleArn: arn:aws:iam::123456789012:role/ordersApiTaskRole
image: 123456789012.dkr.ecr.eu-west-2.amazonaws.com/orders-api:1.0.0
containerPort: 8080
healthCheckPath: /health
cpu: "512"
memory: "1024"
cpuArchitecture: ARM64
subnets:
  - subnet-0123456789abcdef0
  - subnet-0abcdef0123456789
securityGroups:
  - sg-0123456789abcdef0
minTaskCount: 1
maxTaskCount: 5
scalingMetric: AVERAGE_CPU
scalingTargetValue: 60
environment:
  APP_ENV: production
secrets:
  API_KEY: arn:aws:secretsmanager:eu-west-2:123456789012:secret:orders-api-key
awsLogsConfiguration:
  logGroup: /ecs/express/orders-api
  logStreamPrefix: app
tags:
  app: orders-api
```

The smallest configuration accepted by ecsctl needs `serviceName`, `infrastructureRoleArn`, and exactly one of `image` or `taskDefinitionArn`. AWS may require additional values for your service, image, task definition, cluster, and networking setup. For example, the execution role lets ECS pull images and write logs; the task role grants AWS permissions to your application. Keep secret values in Secrets Manager and put their ARNs in `secrets`.

If you already have a task definition, use its ARN instead of the image and container settings. For example:

```yaml
serviceName: orders-api
cluster: production
infrastructureRoleArn: arn:aws:iam::123456789012:role/ECSExpressInfrastructureRole
taskDefinitionArn: arn:aws:ecs:eu-west-2:123456789012:task-definition/orders-api:4
```

Save this as a YAML file too, then pass its path with `-f`. Do not combine `taskDefinitionArn` with any image or container fields listed above.

## Create and check the service

Create the service and wait for ECS to report it active:

```bash
ecsctl express create -f express.yaml --wait
```

The `-f` flag points to the file you saved. `--wait` keeps the command running until ECS reports the service active, then prints the service ingress URL when available. Without `--wait`, the command returns after the create request succeeds.

To find the service later, list Express services in the cluster:

```bash
ecsctl express list --cluster production
```

Copy the service ARN from the output to inspect or update it:

```bash
ecsctl express describe --service-arn <service-arn>
ecsctl express update --service-arn <service-arn> -f express.yaml --wait
```

## Reconcile changes with `apply`

Use `apply` when you want the file to describe the settings ecsctl should maintain. It creates the service if it is missing. If the service already exists, ecsctl compares configured fields and updates the service only when they differ. Fields omitted from the file are left alone.

Preview the changes first:

```bash
ecsctl apply -f express.yaml --dry-run
```

Apply them and wait for the service to become active:

```bash
ecsctl apply -f express.yaml --wait
```

You can rerun `apply` after changing the file. For an existing service, the service name and cluster cannot be changed in place, and the infrastructure role cannot be replaced in place; create a replacement service for those changes.

## Start from an existing Express service

If you want to manage a service that already exists, first configure an [S3 remote state context](state.md). Import the service so ecsctl can capture its current configuration, then save that configuration to a file:

```bash
ecsctl state import express orders-api --cluster production --region eu-west-2
ecsctl state config express orders-api --cluster production > express.yaml
```

Open `express.yaml`, review the values, then use it with `ecsctl apply`. Imported configuration can include environment values returned by ECS. Review the file before storing it in source control, and keep secrets in Secrets Manager.

## Delete a service

Deleting an Express service asks ECS to remove the service and its managed resources. Confirm the ARN before running the command:

```bash
ecsctl express delete --service-arn <service-arn>
```

## Command summary

```bash
ecsctl express create -f express.yaml [--wait]
ecsctl express update --service-arn <service-arn> -f express.yaml [--wait]
ecsctl express describe --service-arn <service-arn>
ecsctl express delete --service-arn <service-arn>
ecsctl express list [--cluster <cluster-name>]
```
