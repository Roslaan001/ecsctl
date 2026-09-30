# Remote State Management

`ecsctl` supports storing and tracking your managed resource states remotely in an Amazon S3 bucket. This serves as a source of truth, enabling collaboration and tracking which resources are managed by `ecsctl` without relying on local files. It is conceptually similar to Terraform state.

Local configurations and active contexts are saved in your home directory at `~/.ecsctl/config.yaml`.

---

## 1. Initialize State Context (`state init`)

Create or link an S3 bucket to act as your remote state backend for a specific context (e.g. staging, production). The tool will verify the bucket's existence (or create it if needed), enable S3 versioning, and configure the local context.

```bash
ecsctl state init --context <context-name> --bucket <s3-bucket-name> --region <aws-region> [flags]
```

### Examples

Initialize a context for staging:
```bash
ecsctl state init --context staging --bucket my-ecsctl-staging-state --region eu-west-1
```

Initialize a context for production, overriding the default AWS profile:
```bash
ecsctl state init --context prod --bucket my-ecsctl-prod-state --region us-east-1 --profile prod-profile
```

---

## 2. Managing Contexts

You can manage multiple state backends (contexts) and switch between them easily.

### List Contexts
List all configured contexts on your local system. The active context is indicated by an asterisk (`*`):

```bash
ecsctl state list-contexts
```

### Switch Contexts
Switch the active context to change which S3 bucket you are querying and applying updates to:

```bash
ecsctl state use-context staging
```

---

## 3. Importing Existing Resources (`state import`)

If you have pre-existing AWS ECS clusters or services that were created manually or by other tools, you can import them into `ecsctl`'s state management tracking.

```bash
# Import an existing cluster into current active context
ecsctl state import cluster my-existing-cluster --region us-east-1

# Import an existing service into current active context
ecsctl state import service my-existing-service --cluster my-existing-cluster --region us-east-1

# Import an existing Express service by name or ARN
ecsctl state import express my-api --cluster my-existing-cluster --region us-east-1
```

Once imported, `ecsctl` will begin tracking these resources, and you can manage them using `ecsctl apply` and describe/list commands.

Import captures the observed cluster, service, or Express configuration in remote state. Print it with `state config` to create a YAML file to review and manage declaratively:

```bash
ecsctl state config cluster my-existing-cluster
ecsctl state config service my-existing-service --cluster my-existing-cluster
ecsctl state config express my-api --cluster my-existing-cluster
```

The configuration may contain environment values returned by ECS. Review it before storing it in source control and keep secrets in Secrets Manager.

---

## 4. Show Active State (`state show`)

Print detailed information about the active context, including the S3 bucket configuration path and tracked clusters, services, and Express services:

```bash
ecsctl state show
```
