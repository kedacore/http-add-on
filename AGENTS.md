# KEDA HTTP Add-on

KEDA HTTP Add-on enables autoscaling HTTP workloads on Kubernetes (including to/from zero) based on incoming traffic.
It extends [KEDA](https://keda.sh) with HTTP-aware scaling.

This file gives AI coding agents (Claude Code, Codex, Cursor, Copilot, Aider, etc.) the context and rules they need when working in this repository.
It complements [`CONTRIBUTING.md`](CONTRIBUTING.md), which remains the source of truth for humans.
Agents must respect every rule there as well.

If a rule here conflicts with `CONTRIBUTING.md`, follow `CONTRIBUTING.md` and flag the discrepancy in the PR description.

## Architecture

Three components, each a separate Go binary:

- **Operator** (`operator/`) — Kubernetes controller managing `InterceptorRoute` and `HTTPScaledObject` CRDs.
- **Interceptor** (`interceptor/`) — HTTP reverse proxy that routes requests and tracks pending request counts for scaling decisions.
- **Scaler** (`scaler/`) — gRPC service implementing the KEDA external scaler protocol. Aggregates queue metrics from interceptors.

Shared libraries live in `pkg/` (routing, queue, k8s helpers, observability, utilities).

Kubernetes manifests (CRDs, RBAC, deployments) live in `config/` with kustomize overlays per component.
Container images are built with [ko](https://ko.build/), not Dockerfiles.
User-facing docs live in a separate repo ([kedacore/keda-docs](https://github.com/kedacore/keda-docs)).
The Helm chart is maintained separately in [kedacore/charts](https://github.com/kedacore/charts).

## Commands

```bash
make test              # Unit tests (go test ./...)
make lint              # Lint (golangci-lint) — covers govet, staticcheck, gosec, gofumpt, and more
make lint-fix          # Lint and auto-fix
make generate          # Regenerate DeepCopy methods + CRDs + RBAC (run after changing API types)
make verify-manifests  # Verify generated manifests are up to date
make deploy            # Build and deploy all components to cluster (requires ko + KO_DOCKER_REPO)
make e2e-setup         # Install e2e dependencies (KEDA, cert-manager, etc.) + deploy
make e2e-test          # Run e2e tests (requires deployed cluster)
```

## Writing code

- Follow the existing patterns in the package you are editing.
  Do not introduce new abstractions, frameworks, or dependencies without justification in the PR description.
- Run `make lint-fix` after changes — it covers formatting, vetting, and linting in one step (see `.golangci.yml` for the full list).
- When modifying types in `operator/apis/http/`, run `make generate` and commit the generated files.
- Use inclusive language.
  The pre-commit hook enforces this; use terms like "deny_list" and "allow_list".

## Testing

- **Unit tests**: `*_test.go` files alongside source. Run with `make test`.
- **E2e tests**: `test/e2e/` organized by profile (`default/`, `observability/`, `tls/`).
  Shared helpers in `test/helpers/`.
  Require a running cluster with KEDA and dependencies deployed (`make e2e-setup`).
- Run a specific e2e test: `make e2e-test RUN=TestColdStart`
- Run a specific profile: `make e2e-test PROFILE=tls`
- Bug fixes should add a regression test that fails without the fix.
- New behavior in existing code should be covered by unit tests.
- New features or significant changes should include e2e tests.
  See [`docs/developing.md`](docs/developing.md#running-e2e-tests) for how to write and run them.
- Do not delete existing tests to make a build green.
  If a test is genuinely wrong, explain why in the PR description.
- Do not weaken assertions (e.g. replacing exact checks with `assert.NotNil`) just to make a flaky test pass.

## Commit requirements

- **Every commit must be signed off** (DCO). Use `git commit -s`. The `Signed-off-by:` trailer must match the author. CI rejects PRs with unsigned commits.
- Follow [Conventional Commits](https://www.conventionalcommits.org/): `<type>[scope]: <description>`.
  Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`.
  Scopes (optional): `interceptor`, `operator`, `scaler`.
- Do not commit generated files that are not produced by the documented `make` targets above.
- Do not commit secrets, credentials, `.env` files, or large binaries.

## Changelog

Every user-visible change must be added to [`CHANGELOG.md`](CHANGELOG.md) under the `## Unreleased` section.
The pre-commit hook [`hack/validate-changelog.sh`](hack/validate-changelog.sh) verifies this.

Rules (from [`CONTRIBUTING.md`](CONTRIBUTING.md#updating-the-changelog) and enforced by [`hack/validate-changelog.sh`](hack/validate-changelog.sh)):

- Place the entry under the correct subsection: `### Breaking Changes`, `### New`, `### Improvements`, `### Fixes`, `### Deprecations`, or `### Other`.
- Format: `- **<Component>**: <Description> ([#<ID>](https://github.com/kedacore/http-add-on/issues/<ID>))`.
  - Use `**General**:` for cross-cutting changes; these go at the top of the subsection.
  - Otherwise use the component name: `**Interceptor**`, `**Operator**`, or `**Scaler**`.
- Entries are sorted **alphabetically** within each subsection, with `General` always first.
- `<ID>` should preferably link to an issue; if none exists, link the PR.
- Internal-only changes (refactors, test-only changes, CI tweaks) do **not** require a changelog entry.

## Pull request rules

- **Do not delete or modify the checklist** in [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md). When opening a PR, keep every checklist item and tick off only the boxes that genuinely apply to the change.
- Keep the `Fixes #` line and fill it in when there is a related issue or PR.
- Write a clear description of *what* changed and *why*. Do not leave the template description empty.
- One logical change per PR. Do not bundle unrelated refactors with feature work or bug fixes.
- For non-trivial work (larger changes, new features), there should be an existing GitHub Issue or Discussion first. Small fixes (typos, minor bug fixes) may go directly to a PR.
- Do not "drive-by" reformat, rename, or restructure code outside the scope of the requested change.
- Do not bump dependencies unless the task requires it.
- Do not change CI workflows, release tooling, or governance files unless explicitly asked.
- Behavior or UX changes require a matching docs PR against [`kedacore/keda-docs`](https://github.com/kedacore/keda-docs). Link it in the PR description.
- Manifest changes that affect deployment require a matching PR against [`kedacore/charts`](https://github.com/kedacore/charts).

## When in doubt

Stop and ask the human reviewer rather than guessing.
It is better to leave a `TODO` and surface the question in the PR description than to invent behavior, fabricate API names, or silence failing checks.

## See also

- [docs/developing.md](docs/developing.md) — prerequisites, ko workflow, and writing e2e tests
- [CONTRIBUTING.md](CONTRIBUTING.md) — PR guidelines, changelog format, DCO sign-off details
