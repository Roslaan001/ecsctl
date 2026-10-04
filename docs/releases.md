# Release notes and upgrades

Release Please creates a reviewed release pull request from the Conventional Commits merged to the default branch. After that pull request is merged, ecsctl publishes a GitHub release with its version and release notes.

The workflow keeps an open release pull request updated as the default branch advances, including maintenance changes merged while it is open.

## Release history

- [Browse published releases](https://github.com/Roslaan001/ecsctl/releases) for version-specific notes and downloadable binaries.
- Read the complete [CHANGELOG.md](https://github.com/Roslaan001/ecsctl/blob/main/CHANGELOG.md) for the project history.

The release notes group changes into features, bug fixes, performance, documentation, maintenance, and reverts. Scopes such as `security` remain visible alongside the change summary. Chore commits appear under Maintenance; build, CI, test, style, and refactor commits remain hidden.

## Upgrade guidance

Read the notes for the target version before upgrading. Breaking changes are marked in the release notes. If a release needs migration steps beyond the short breaking-change description, this page will include a section for that version and the release notes will link here.

There are no separate migration steps documented for the releases currently listed in the changelog. Future version-specific guidance should use this format:

### vX.Y.Z

- **Who is affected:** Describe the users or configurations that need to change.
- **Before upgrading:** List required preparation, if any.
- **After upgrading:** Give the migration steps and how to verify them.
