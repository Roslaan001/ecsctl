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

Do not put GitHub issue-closing references in commit messages. Keywords such as `close`, `fix`, or `resolve` followed by an issue number can close an issue when the commit is merged. Put issue-closing references in the pull request description instead. A Conventional Commit such as `fix(state): prevent stale lock removal` is fine when it has no issue-closing reference. Avoid `@mentions` in commit messages because GitHub can notify mentioned users again whenever the pull request is updated.

The commit message workflow checks each commit's subject format and full message for issue-closing references on pushes and pull requests targeting `main` or `master`, including pull requests opened from forks. On a new branch's first push, it checks commits against `main`. Older commits already on an existing branch are grandfathered when this policy is introduced.

To reject invalid messages before creating a commit, enable the repository hook once:

```bash
git config core.hooksPath .githooks
```

## Pull requests

Use the pull request template to describe the change, the validation performed, documentation updates, and operational impact. Changes are routed to the project maintainer listed in `.github/CODEOWNERS`. The existing GitHub Actions workflows run the build and tests, lint, vulnerability scan, supported-platform builds, documentation checks, and Workers deployment checks.

To make these checks block merges, configure the repository's `main` branch rules to require the `Commit messages / Validate commit messages` check along with the required CI and documentation checks. GitHub repository settings are managed separately from the workflow files.
