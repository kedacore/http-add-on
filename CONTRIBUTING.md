# Contributing to KEDA HTTP Add-on

Thanks for helping make the KEDA HTTP Add-on better 😍.

There are many areas we can use contributions - ranging from code, documentation, feature proposals, issue triage, samples, and content creation.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Getting Started](#getting-started)
  - [Quick Overview](#quick-overview)
- [Submitting Changes](#submitting-changes)
  - [Commit Conventions](#commit-conventions)
  - [Updating the Changelog](#updating-the-changelog)
  - [Pull Request Guidelines](#pull-request-guidelines)
- [Developer Certificate of Origin: Signing your work](#developer-certificate-of-origin-signing-your-work)
  - [Every commit needs to be signed](#every-commit-needs-to-be-signed)
  - [I didn't sign my commit, now what?](#i-didnt-sign-my-commit-now-what)
- [Communication](#communication)
- [Project Governance](#project-governance)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Getting Started

If you're new to the project, issues labeled [`good first issue`](https://github.com/kedacore/http-add-on/labels/good%20first%20issue) and [`help wanted`](https://github.com/kedacore/http-add-on/labels/help%20wanted) are good starting points.

For small fixes (typos, minor bug fixes), feel free to open a pull request directly.
For larger changes or new features, please [open an issue](https://github.com/kedacore/http-add-on/issues/new) or [start a discussion](https://github.com/kedacore/http-add-on/discussions/new/choose) first so the approach can be agreed on before you invest significant effort.

### Quick Overview

1. [Fork](https://github.com/kedacore/http-add-on/fork) and clone the repository
2. Set up your development environment — see [docs/developing.md](./docs/developing.md) for prerequisites and tooling
3. Create a branch for your changes (`git switch -c my-change`)
4. Make your changes and verify them:

   ```bash
   make lint       # runs linter
   make lint-fix   # runs linter and auto-fixes issues
   make test       # runs unit tests
   ```

5. Commit using [conventional commits](#commit-conventions) with a DCO sign-off (`git commit -s`)
6. Update the [changelog](#updating-the-changelog) for user-facing changes
7. Open a pull request — see [pull request guidelines](#pull-request-guidelines)

## Submitting Changes

### Commit Conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```text
<type>[scope]: <description>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`
Scopes (optional): `interceptor`, `operator`, `scaler`

Examples:

- `feat(interceptor): add support for gRPC routing`
- `fix(operator): handle nil pointer in reconcile loop`
- `chore: bump Go version to 1.23`

All commits must also include a DCO sign-off (`-s`) — see [Developer Certificate of Origin](#developer-certificate-of-origin-signing-your-work).

### Updating the Changelog

For user-facing changes, add an entry to the `Unreleased` section of [CHANGELOG.md](./CHANGELOG.md) using this format:

```text
- **Component**: Description ([#ISSUE](https://github.com/kedacore/http-add-on/issues/ISSUE))
```

Where component is `General`, `Interceptor`, `Operator`, or `Scaler`. Run `./hack/validate-changelog.sh` to verify the entry.

### Pull Request Guidelines

- Keep pull requests focused — one concern per PR.
- Include tests for new functionality or bug fixes.
- Reference the issue being fixed (e.g., `Fixes #123`).
- Ensure `make test` passes before submitting.
- Include documentation updates in the same PR for changes that affect behavior, and if appropriate, in [the KEDA docs repository](https://github.com/kedacore/keda-docs).
- For end-to-end tests, see [tests/README.md](./tests/README.md).

## Developer Certificate of Origin: Signing your work

### Every commit needs to be signed

The Developer Certificate of Origin (DCO) is a lightweight way for contributors to certify that they wrote or otherwise have the right to submit the code they are contributing to the project.
Here is the full text of the DCO, reformatted for readability:

```text
By making a contribution to this project, I certify that:

    (a) The contribution was created in whole or in part by me and I have the right to submit it under the open source license indicated in the file; or

    (b) The contribution is based upon previous work that, to the best of my knowledge, is covered under an appropriate open source license and I have the right under that license to submit that work with modifications, whether created in whole or in part by me, under the same open source license (unless I am permitted to submit under a different license), as indicated in the file; or

    (c) The contribution was provided directly to me by some other person who certified (a), (b) or (c) and I have not modified it.

    (d) I understand and agree that this project and the contribution are public and that a record of the contribution (including all personal information I submit with it, including my sign-off) is maintained indefinitely and may be redistributed consistent with this project or the open source license(s) involved.
```

Contributors sign-off that they adhere to these requirements by adding a `Signed-off-by` line to commit messages.

```text
This is my commit message

Signed-off-by: Random J Developer <random@developer.example.org>
```

Git even has a `-s` command line option to append this automatically to your commit message:

```console
git commit -s -m 'This is my commit message'
```

Pull requests are automatically checked for valid Signed-off-by lines.

### I didn't sign my commit, now what?

Sign all commits on your branch and force push:

```console
git rebase main --exec 'git commit --amend --no-edit -s'
git push --force
```

## Communication

- [GitHub Discussions](https://github.com/kedacore/http-add-on/discussions) — questions, ideas, and general conversation
- [GitHub Issues](https://github.com/kedacore/http-add-on/issues) — bug reports and feature requests
- [Kubernetes Slack](https://kubernetes.slack.com/messages/CKZJ36A5D) — `#keda` channel ([join here](https://slack.k8s.io))

## Project Governance

See the [KEDA governance documentation](https://github.com/kedacore/governance).
