# Repository setup required for maintainers

## Release Please

No personal access token is required. Release Please uses the workflow's `GITHUB_TOKEN` to open or update its release pull request. Since GitHub does not start new workflows for tags created with `GITHUB_TOKEN`, this workflow runs tests and builds release artifacts itself after creating a release. Merging a release pull request also runs normal push CI on `main`.

## ECS Express AWS smoke test

The manually dispatched `ECS Express AWS smoke test` workflow assumes an AWS role through GitHub OIDC. Add the repository secret `AWS_SMOKE_ROLE_ARN` containing the role ARN. Configure its trust policy for this repository and the `main` branch, with the GitHub OIDC audience `sts.amazonaws.com`.

The role needs only the permissions required by the smoke lifecycle: create, describe, and delete an ECS Express Gateway service, plus `iam:PassRole` for the supplied infrastructure and task execution roles. The Express service’s infrastructure role must be able to provision its managed AWS resources. Supply an existing test cluster, image, and optional subnets/security groups in the manual workflow inputs. The test caps the service at one task and requests deletion when the test exits.

This workflow is opt-in because it creates real AWS resources and can incur charges. Normal pull-request CI uses dummy credentials and Floci.

## Merge protections

The last repository-settings check found no branch protection rules. GitHub currently rejects branch-protection API access for this private repository with a plan-level 403. Upgrade the account to a plan that supports private-repository protections, then require pull requests and these checks on `main`:

- Build & Test
- Lint
- Cross-build matrix
- Go vulnerability scan
- Floci ECS integration
- Docs build

Do not make the repository public as a workaround without an explicit decision to publish it.
