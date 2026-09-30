# ECS Express Mode

`ecsctl express` manages ECS Express Mode services. ECS provisions and operates the service's managed ingress, load balancer, scaling, logs, and alarms. Use a service YAML file:

```yaml
serviceName: agrogpt-api
cluster: production
infrastructureRoleArn: arn:aws:iam::123456789012:role/ECSExpressInfrastructureRole
executionRoleArn: arn:aws:iam::123456789012:role/ecsTaskExecutionRole
taskRoleArn: arn:aws:iam::123456789012:role/agrogptTaskRole
image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/agrogpt-api:latest
containerPort: 8080
healthCheckPath: /health
cpu: "512"
memory: "1024"
cpuArchitecture: ARM64
subnets: [subnet-0123456789abcdef0, subnet-0abcdef0123456789]
securityGroups: [sg-0123456789abcdef0]
minTaskCount: 1
maxTaskCount: 10
scalingMetric: AVERAGE_CPU
scalingTargetValue: 60
environment:
  APP_ENV: production
secrets:
  API_KEY: arn:aws:secretsmanager:us-east-1:123456789012:secret:agrogpt-api-key
awsLogsConfiguration:
  logGroup: /ecs/express/agrogpt-api
  logStreamPrefix: app
repositoryCredentials:
  credentialsParameter: arn:aws:secretsmanager:us-east-1:123456789012:secret:ecr-pull
tags:
  app: agrogpt
```

Create and wait for ECS to report the service active:

```bash
ecsctl express create -f express.yaml --wait
```

The same YAML can be reconciled with `ecsctl apply -f express.yaml`; apply creates the service when it is missing, updates configured fields only when they differ, and leaves unspecified fields alone. Add `--dry-run` to see whether configured fields would change without sending an update. `ecsctl express list --cluster production` lists Express services in a cluster.

For an existing task definition, replace the image/role/CPU/memory fields with `taskDefinitionArn`. Express services require an infrastructure role with permissions for the managed networking, load balancing, and scaling resources. Update a service using `ecsctl express update --service-arn <arn> -f express.yaml`; inspect or delete it with `ecsctl express describe` and `ecsctl express delete`.

Supported operations:

```bash
ecsctl express create -f express.yaml [--wait]
ecsctl express update --service-arn <arn> -f express.yaml [--wait]
ecsctl express describe --service-arn <arn>
ecsctl express delete --service-arn <arn>
ecsctl express list [--cluster <name>]
```

`ecsctl state import express <name-or-arn> --cluster <name>` records an existing Express service and captures its observed configuration. `ecsctl state config express <name> --cluster <name>` prints a stored configuration that you can save as a YAML starting point. The same `state config` command supports imported clusters and regular ECS services. Review imported values before committing them, especially environment values; use Secrets Manager references for secrets.
