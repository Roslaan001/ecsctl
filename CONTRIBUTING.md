# Contributing

## Commit messages

Use Conventional Commits. Release Please reads these messages to prepare release notes and version bumps.

```text
feat: add service import
fix(state): prevent stale lock removal
docs: explain drift checks
ci: validate commit messages
```

Use one of `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, or `revert`. Keep the subject to 72 characters. Add a scope when it helps, such as `fix(state): ...`; mark breaking changes with `!`, such as `feat!: ...`.

Pull request titles follow the same Conventional Commit format. The PR metadata check also requires non-empty Summary, Validation, and Operational impact sections in the description.

Do not put GitHub issue-closing references in commit messages. Keywords such as `close`, `fix`, or `resolve` followed by an issue number can close an issue when the commit is merged. Put issue-closing references in the pull request description instead. A Conventional Commit such as `fix(state): prevent stale lock removal` is fine when it has no issue-closing reference. Avoid `@mentions` in commit messages because GitHub can notify mentioned users again whenever the pull request is updated.

Commit bodies are optional and may use free-form text. The commit message workflow checks each commit's subject format and full message for issue-closing references and `@mentions` on pushes and pull requests targeting `main` or `master`, including pull requests opened from forks. The ECSCTL PR bot applies `ecsctl/invalid-commit-message` when a PR contains one and removes it after the messages are corrected. On a new branch's first push, the workflow checks commits against `main`. Older commits already on an existing branch are grandfathered when this policy is introduced.

To reject invalid messages before creating a commit, enable the repository hook once:

```bash
git config core.hooksPath .githooks
```

## Pull requests

Use the pull request template to describe the change, the validation performed, documentation updates, and operational impact. Changes are routed to the project maintainer listed in `.github/CODEOWNERS`. The existing GitHub Actions workflows run the build and tests, Go lint, workflow and shell-script lint, PR metadata and commit-message validation, vulnerability and secret scans, supported-platform builds, CodeQL analysis for Go and Actions workflows, and documentation checks.

The `protect-main` ruleset requires the checks that validate code, commit messages, PR metadata, and automation before merge.

## Tests

Run the unit suite with:

```bash
go test ./...
```

The live EC2 cluster smoke test is opt-in. It creates a temporary ECS cluster, one `t3.small` instance, an Auto Scaling group, and an IAM instance role in the default VPC, then removes those resources during cleanup. EC2 charges apply while the instance is running. Set the expected 12-digit account ID before enabling the test:

```bash
AWS_PROFILE=test \
ECSCTL_AWS_EC2_EXPECTED_ACCOUNT=123456789012 \
ECSCTL_AWS_EC2_SMOKE=1 \
go test -tags=integration ./pkg/aws -run '^TestAWSEC2ClusterLifecycle$' -count=1 -v
```

The test verifies the caller account, active cluster and capacity provider, default EC2 capacity strategy, and registered instance. It fails closed when the profile, expected account, or smoke-test gate is missing. If cleanup reports an error, inspect ECS, EC2 Auto Scaling, launch templates, and the `ecsctl:cluster` IAM resources for leftovers.

The live Fargate smoke test runs one short-lived task in a temporary cluster, checks its Fargate launch type and successful exit, then deregisters its task definition and deletes the cluster. It uses the default VPC's public subnets and security group, so the task must be able to reach the public image registry. A task execution role ARN can be supplied if the account requires one:

```bash
AWS_PROFILE=test \
ECSCTL_AWS_FARGATE_EXPECTED_ACCOUNT=123456789012 \
ECSCTL_AWS_FARGATE_SMOKE=1 \
go test -tags=integration ./pkg/aws -run '^TestAWSFargateTaskLifecycle$' -count=1 -v
```

Set `ECSCTL_AWS_FARGATE_IMAGE` to use another public image or `ECSCTL_AWS_FARGATE_EXECUTION_ROLE_ARN` when required. This smoke test creates billable Fargate task runtime and cleans up its ECS resources.
