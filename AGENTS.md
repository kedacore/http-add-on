# KEDA HTTP Add-on

KEDA HTTP Add-on enables autoscaling HTTP workloads on Kubernetes (including to/from zero) based on incoming traffic.
It extends [KEDA](https://keda.sh) with HTTP-aware scaling.

## Architecture

Three components, each a separate Go binary:

- **Operator** (`operator/`) — Kubernetes controller managing `InterceptorRoute` CRDs.
- **Interceptor** (`interceptor/`) — HTTP reverse proxy that routes requests and tracks pending request counts for scaling decisions.
- **Scaler** (`scaler/`) — gRPC service implementing the KEDA external scaler protocol. Aggregates queue metrics from interceptors.

Shared libraries live in `pkg/` (routing, queue, k8s helpers, observability, utilities).

Kubernetes manifests (CRDs, RBAC, deployments) live in `config/` with kustomize overlays per component.
Container images are built with [ko](https://ko.build/), not Dockerfiles.
All ko commands require `KO_DOCKER_REPO` to be set (see "Building and deploying").
User-facing docs live in a separate repo ([kedacore/keda-docs](https://github.com/kedacore/keda-docs)) and may need updates for behavior changes.
The Helm chart is maintained separately in [kedacore/charts](https://github.com/kedacore/charts) and may need updates for new fields or configuration.

## Commands

```bash
make build             # Build
make test              # Unit tests
make lint              # Lint (golangci-lint) — covers govet, staticcheck, gosec, gofumpt, and more
make lint-fix          # Lint and auto-fix
make generate          # Regenerate DeepCopy methods + CRDs + RBAC (run after changing API types)
make verify-manifests  # Verify generated manifests are up to date
make deploy            # Build and deploy all components to cluster (requires ko + KO_DOCKER_REPO)
make e2e-setup         # Install e2e dependencies (KEDA, cert-manager, etc.) + deploy (requires KO_DOCKER_REPO)
make e2e-test-images   # Build test images under test/images/ (needed for gRPC tests)
make e2e-test          # Run e2e tests (requires deployed cluster)
```

## Building and deploying

```bash
export KO_DOCKER_REPO=kind.local     # for KinD
export KO_DOCKER_REPO=localhost:5001 # for local registry

make deploy                          # deploy all components
make deploy-interceptor              # deploy single component
kubectl rollout status deployment/keda-add-ons-http-interceptor -n keda
```

## Writing code

- Use Go standard library for testing (`testing` package, not testify/gomega/ginkgo) unless there are strong reasons.
- Follow the existing patterns in the package you are editing.
  Do not introduce new abstractions, frameworks, or dependencies without justification in the PR description.
- Run `make lint-fix` after changes — it covers formatting, vetting, and linting in one step (see `.golangci.yml` for the full list).
- When modifying types in `operator/apis/http/`, run `make generate` and commit the generated files.
- Use inclusive language, e.g. `allowlist` and `denylist`.
- In Markdown files, put each sentence on its own line.

## Testing

- **Unit tests**: `*_test.go` files alongside source. Run with `make test`.
- **E2e tests**: `test/e2e/` organized by profile (`default/`, `observability/`, `tls/`).
  Shared helpers in `test/helpers/`.
  Require a running cluster with KEDA and dependencies deployed (`make e2e-setup`).
- Run a specific e2e test: `make e2e-test RUN=TestColdStart`
- Run a specific profile: `make e2e-test PROFILE=tls`
- Work test-first: write a failing test (a regression test for bug fixes), confirm it fails for the expected reason, then make it pass (red-green).
- New behavior must be covered by unit tests.
- New features or significant changes should include e2e tests.
  See [`docs/developing.md`](docs/developing.md#running-e2e-tests) for how to write and run them.
- Do not delete existing tests to make a build green.
  If a test is genuinely wrong, explain why in the PR description.
- Do not weaken assertions (e.g. replacing exact value checks with nil checks) just to make a flaky test pass.

## Commit requirements

1. **Lint**: `make lint` must pass (auto-fix with `make lint-fix`)
2. **Changelog**: update the `Unreleased` section in `CHANGELOG.md` for user-facing changes, then validate with `./hack/validate-changelog.sh`
3. **Conventional Commits**: `<type>(<scope>): <description>` with optional scope (types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`; scopes: `interceptor`, `operator`, `scaler`)
4. **DCO sign-off**: every commit must use `git commit -s`

## See also

- [docs/developing.md](./docs/developing.md) — prerequisites, ko workflow, and writing e2e tests
- [CONTRIBUTING.md](./CONTRIBUTING.md) — PR guidelines, changelog format, DCO sign-off details
