# Resource Inspection & Querying

`ecsctl` provides inspection commands (`list` and `describe`) to query the live status of your ECS clusters, services, and tasks, or to view what resources are registered in your active state context.

---

## 🔍 Listing Resources (`list`)

The `list` command displays lists of clusters, services, and tasks. If a remote state context is active, `list clusters` and `list services` show the saved inventory by default. Add `--live` to query AWS directly and see current resources, including resources changed or deleted outside ecsctl. Without a configured state context, the commands query AWS directly.

```bash
ecsctl list clusters --live --region eu-west-2
ecsctl list services --cluster production --live --region eu-west-2
ecsctl list tasks --cluster production --wide --region eu-west-2
ecsctl list task-definitions --status ACTIVE --wide --region eu-west-2
ecsctl list services --cluster production --output json
ecsctl list tasks --cluster production --service api --desired-status STOPPED --sort startedAt --limit 20
ecsctl list task-definitions --family api --sort registeredAt --limit 10
```

Live cluster lists show active services, running and pending tasks, and capacity providers. State-backed cluster lists also show the recorded creation time for clusters created through ecsctl. Live service lists include deployment health/state, desired/running/pending counts, launch type, task definition, and creation time. Task lists include health, desired and last status, service, launch type, CPU, memory, and timestamps. Task definition lists include CPU, memory, and registration time. Add `--wide` to include full ARNs.

All list commands accept `--output table|json`, `--sort FIELD`, and `--limit N` (`0` means no limit). `--sort` accepts a displayed field name, such as `name`, `createdAt`, `startedAt`, or `registeredAt`. Name and family filters match prefixes. Cluster and service `--status` filters query live AWS, as status is not part of the saved inventory. JSON output always includes ARN fields and emits only JSON on stdout.

### 1. List Clusters
List all ECS clusters:

```bash
ecsctl list clusters [flags]
```

### 2. List Services
List all services running in a specific cluster:

```bash
ecsctl list services --cluster <cluster-name> [flags]
```

* `--cluster` (string, required): The name of the ECS cluster to query.

### 3. List Tasks
List all tasks running in a cluster. You can optionally filter the tasks to show only those belonging to a specific service:

```bash
# List all tasks in a cluster
ecsctl list tasks --cluster <cluster-name>

# List only tasks running in a specific service
ecsctl list tasks --cluster <cluster-name> --service <service-name>
```

* `--cluster` (string, required): The name of the ECS cluster.
* `--service` (string, optional): Filter tasks by service name.
* `--status` (string): Filter by last known task status.
* `--desired-status` (string): Filter by desired ECS task status.
* `--launch-type` (string): Filter by launch type.

### 4. List Task Definitions

List task definition families and revisions. The default status is `ACTIVE`:

```bash
ecsctl list task-definitions [--status ACTIVE|INACTIVE|DELETE_IN_PROGRESS] [--family PREFIX] [--wide]
```

---

## 📄 Describing Resources (`describe`)

The `describe` command prints detailed runtime configurations, status metrics, and resource metadata directly from AWS.

### 1. Describe Cluster
Describe a specific ECS cluster by passing its name as an argument:

```bash
ecsctl describe cluster <cluster-name> [flags]
```

#### Example
```bash
ecsctl describe cluster my-cluster
```

### 2. Describe Service
Describe an ECS service. You must pass the service name as an argument and specify the cluster using the `--cluster` flag:

```bash
ecsctl describe service <service-name> --cluster <cluster-name> [flags]
```

#### Example
```bash
ecsctl describe service my-service --cluster my-cluster
```

### 3. Describe Task

Inspect a task's status, stop reason, container exit codes and reasons, timestamps, and network attachments:

```bash
ecsctl describe task <task-id-or-arn> --cluster <cluster-name>
```
