# Task definitions and one-off tasks

Register a standard ECS task definition from the JSON shape accepted by `RegisterTaskDefinition`:

```bash
ecsctl register task-definition -f task-definition.json
```

The JSON should include `family` and `containerDefinitions`; the other supported ECS task definition fields pass through to the AWS API. `ecsctl deploy` copies all registrable task-level settings, including runtime platform, ephemeral storage, fault injection, placement constraints, proxy settings, and tags, while changing only the selected container image.

Run a standalone Fargate task. Subnets and security groups are needed for tasks using `awsvpc` networking:

```bash
ecsctl run-task --cluster production --task-definition batch-job:4 \
  --count 1 --subnets subnet-a,subnet-b --security-groups sg-a
```

Stop a task by ID or ARN:

```bash
ecsctl stop-task --cluster production --task arn:aws:ecs:us-east-1:123456789012:task/production/abc123
```

These commands use ECS task APIs directly and do not create a service or keep the task running after it exits.
