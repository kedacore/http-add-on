# Changelog

<!--
    New changelog entries must be inline with our changelog guidelines.
    Please refer to https://github.com/kedacore/keda/blob/main/CONTRIBUTING.md#Changelog to learn more.
-->

This changelog keeps track of work items that have been completed and are ready to be shipped in the next release.

## History

- [Unreleased](#unreleased)
- [v0.7.0](#v070)
- [v0.6.0](#v060)
- [v0.5.0](#v050)

## Unreleased

### Breaking Changes

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### New

- **General**: Propagate HTTPScaledObject labels and annotations to ScaledObject ([#840](https://github.com/kedacore/http-add-on/issues/840))

### Improvements

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Fixes

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):


Previously announced deprecation(s):

- **General**: Deprecated `KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS` in favor of `KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS` ([#438](https://github.com/kedacore/http-add-on/issues/438))

### Other

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

## v0.7.0

### Breaking Changes

- **General**: `host` field has been removed in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552)|[#888](https://github.com/kedacore/http-add-on/pull/888))

### New

- **General**: Support any resource which implements `/scale` subresource ([#438](https://github.com/kedacore/http-add-on/issues/438))

### Improvements

- **General**: Improve Scaler reliability adding probes and 3 replicas ([#870](https://github.com/kedacore/http-add-on/issues/870))

### Fixes

- **General**: Add new user agent probe ([#862](https://github.com/kedacore/http-add-on/issues/862))
- **General**: Fix external scaler getting into bad state when retrieving queue lengths fails. ([#870](https://github.com/kedacore/http-add-on/issues/870))
- **General**: Increase ScaledObject polling interval to 15 seconds ([#799](https://github.com/kedacore/http-add-on/issues/799))
- **General**: Set forward request RawPath to original request RawPath ([#864](https://github.com/kedacore/http-add-on/issues/864))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: Deprecated `KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS` in favor of `KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS` ([#438](https://github.com/kedacore/http-add-on/issues/438))

### Other

- **General**: Bump golang version ([#853](https://github.com/kedacore/http-add-on/pull/853))

## v0.6.0

### New

- **General**: Add manifests to deploy the Add-on ([#716](https://github.com/kedacore/http-add-on/issues/716))

### Improvements

- **Scaler**: Decrease memory usage by allowing increasing stream interval configuration ([#745](https://github.com/kedacore/http-add-on/pull/745))

### Fixes

- **Interceptor**: Add support for streaming responses ([#743](https://github.com/kedacore/http-add-on/issues/743))
- **Interceptor**: Fatal error: concurrent map iteration and map write ([#726](https://github.com/kedacore/http-add-on/issues/726))
- **Interceptor**: Keep original Host in the Host header ([#331](https://github.com/kedacore/http-add-on/issues/331))
- **Interceptor**: Provide graceful shutdown for http servers on SIGINT and SIGTERM ([#731](https://github.com/kedacore/http-add-on/issues/731))
- **Operator**: Remove ScaledObject `name` & `app` custom labels ([#717](https://github.com/kedacore/http-add-on/issues/717))
- **Scaler**: Provide graceful shutdown for grpc server on SIGINT and SIGTERM ([#731](https://github.com/kedacore/http-add-on/issues/731))
- **Scaler**: Reimplement custom interceptor metrics ([#718](https://github.com/kedacore/http-add-on/issues/718))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

### Other

- **General**: Adding a changelog validating script to check for formatting and order ([#761](https://github.com/kedacore/http-add-on/pull/761))
- **General**: Skip not required CI checks on PRs on new commits ([#801](https://github.com/kedacore/http-add-on/pull/801))

## v0.5.0

### Breaking Changes

None.

### New

- **General**: Log incoming requests using the Combined Log Format ([#669](https://github.com/kedacore/http-add-on/pull/669))
- **Routing**: Add multi-host support to `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))
- **Routing**: Support path-based routing ([#338](https://github.com/kedacore/http-add-on/issues/338))

### Improvements

- **General**: Automatically tag Docker image with commit SHA ([#567](https://github.com/kedacore/http-add-on/issues/567))
- **Operator**: Migrate project to Kubebuilder v3 ([#625](https://github.com/kedacore/http-add-on/issues/625))
- **RBAC**: Introduce fine-grained permissions per component and reduce required permissions ([#612](https://github.com/kedacore/http-add-on/issues/612))
- **Routing**: New routing table implementation that relies on the live state of HTTPScaledObjects on the K8s Cluster instead of a ConfigMap that is updated periodically ([#605](https://github.com/kedacore/http-add-on/issues/605))

### Fixes

- **General**: Changes to HTTPScaledObjects now take effect ([#605](https://github.com/kedacore/http-add-on/issues/605))
- **General**: HTTPScaledObject is the owner of the underlying ScaledObject ([#703](https://github.com/kedacore/http-add-on/issues/703))
- **Controller**: Use kedav1alpha1.ScaledObject default values ([#607](https://github.com/kedacore/http-add-on/issues/607))
- **Routing**: Lookup host without port ([#608](https://github.com/kedacore/http-add-on/issues/608))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

Previously announced deprecation(s):

- None.

### Other

- **General**: Use kubernetes e2e images for e2e test and samples ([#665](https://github.com/kedacore/http-add-on/issues/665))
- **e2e tests**: Use the same e2e system as in core ([#686](https://github.com/kedacore/http-add-on/pull/686))
