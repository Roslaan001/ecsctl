# Remote State Management

Remote state is optional. Without it, ecsctl can still send supported commands to AWS, but it does not keep a shared record of which resources you manage. Set up remote state when you want to share that record across machines or team members.

ecsctl stores remote state in an Amazon S3 bucket. Each named context points to a bucket, a key, a Region, and optionally an AWS profile. The context settings are kept locally in `~/.ecsctl/config.yaml`; the resource records are kept in S3. This is a tracking record, not a replacement for AWS: ECS remains the source of the live resource configuration.

---

## 1. Initialize a state context

Initialize a context before using commands that import resources or read/write remote state. The command checks the S3 bucket and creates it if it does not exist, enables bucket versioning, then saves the context locally and makes it active. You need AWS permissions to inspect or create the bucket and enable versioning.

```bash
ecsctl state init --context <context-name> --bucket <s3-bucket-name> --region <aws-region> [flags]
```

### Examples

Initialize a context for staging. The command also selects it as the active context:
```bash
ecsctl state init --context staging --bucket my-ecsctl-staging-state --region eu-west-1
```

Initialize a context for production, overriding the default AWS profile:
```bash
ecsctl state init --context prod --bucket my-ecsctl-prod-state --region us-east-1 --profile prod-profile
```

---

## 2. Managing Contexts

You can save multiple contexts, for example one for staging and one for production. Commands that read or write remote state use the active context unless you select another with `--context`. The context selects a state bucket; use `--region` and `--profile` to select AWS access for other commands.

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

If you have an ECS cluster or service that was created outside ecsctl, import it before you ask ecsctl to track it. Import does not create or change the AWS resource; it reads the resource from AWS and records its observed configuration in the active S3 state context. An active context is required.

```bash
# Import an existing cluster into current active context
ecsctl state import cluster my-existing-cluster --region us-east-1

# Import an existing service into current active context
ecsctl state import service my-existing-service --cluster my-existing-cluster --region us-east-1

# Import an existing Express service by name or ARN
ecsctl state import express my-api --cluster my-existing-cluster --region us-east-1
```

After import, ecsctl can include the resource in state-based commands. Importing does not create a local configuration file by itself.

### Export a starting configuration

`state config` prints the configuration captured during import. Redirect the output to a local file if you want to edit it and use `apply`:

```bash
ecsctl state config cluster my-existing-cluster > cluster.yaml
ecsctl state config service my-existing-service --cluster my-existing-cluster > service.yaml
ecsctl state config express my-api --cluster my-existing-cluster > express.yaml
```

Review the generated file before applying or committing it. AWS may return environment values as plain text; move sensitive values to Secrets Manager and reference the secret ARN in your configuration.

---

## 4. Show Active State (`state show`)

Print the active context details and the resources recorded in its S3 state:

```bash
ecsctl state show
```
