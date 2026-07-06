# CLI Reference

This page describes all commands, subcommands, and global flags available in `ecsctl`.

---

## Global Flags

The following flags can be supplied to **any** command or subcommand to override configured context or credentials on the fly.

| Flag | Shorthand | Type | Description |
|---|---|---|---|
| `--region` | - | string | AWS region to target (e.g. `us-east-1`). Overrides environment variables. |
| `--profile` | - | string | AWS profile name to authenticate with from `~/.aws/credentials`. |
| `--context` | - | string | Named remote state context to run this command under (temporary override). |

---

## Command Hierarchy

Below is the complete command tree for `ecsctl`.

```text
ecsctl
├── apply            - Create or update resources from a YAML config file
├── create           - Explicitly create resources (cluster, service) from YAML
│   ├── cluster
│   └── service
├── delete           - Delete resources (cluster, service) by name
│   ├── cluster
│   └── service
├── describe         - Show detailed live configuration and stats of a resource
│   ├── cluster
│   └── service
├── list             - List active resources from state (or live AWS)
│   ├── clusters
│   ├── services
│   └── tasks
├── deploy           - Update a service container image tag (rolling deploy)
├── scale            - Scale desired task count of a service
├── logs             - Aggregate and follow service task logs
├── exec             - Open interactive shell or run commands in container task
├── state            - Manage remote state backend contexts
│   ├── init
│   ├── list-contexts
│   ├── use-context
│   ├── import
│   └── show
└── version          - Print build version and compilation information
```

---

## Detailed Command Specifications

### `apply`
Create or update resources based on YAML configs.
* **Flags**:
  * `-f, --file` (string, required): Path to YAML config file.
  * `--dry-run` (boolean): Show what changes would occur without applying them.
  * `--wait` (boolean): Wait for resource to reach a stable state before exiting.

### `deploy`
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `--image` (string, required): New container image tag (e.g. `nginx:latest`).
  * `--container` (string): Target container name in the task definition (defaults to first container).

### `scale`
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `--desired` (integer, required): Desired number of tasks to run.

### `logs`
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `-f, --follow` (boolean): Stream logs in real-time.
  * `--tail` (integer): Number of lines to show from the end (defaults to all).

### `exec`
* **Arguments**: `<service-name>`
* **Flags**:
  * `--cluster` (string, required): Target cluster name.
  * `--container` (string): Target container name in the task definition.
  * `--command` (string): Command to execute (defaults to shell).
