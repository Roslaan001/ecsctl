# Resource Inspection & Querying

`ecsctl` provides inspection commands (`list` and `describe`) to query the live status of your ECS clusters, services, and tasks, or to view what resources are registered in your active state context.

---

## 🔍 Listing Resources (`list`)

The `list` command displays lists of clusters, services, and tasks. If a remote state context is active, `list clusters` and `list services` will fetch details from the state backend. Otherwise, they will query your live AWS account.

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
