# Changelog

<!--
    New changelog entries must be inline with our changelog guidelines.
    Please refer to https://github.com/kedacore/keda/blob/main/CONTRIBUTING.md#Changelog to learn more.
-->

This changelog keeps track of work items that have been completed and are ready to be shipped in the next release.

## History

- [Unreleased](#unreleased)
- [v0.5.0](#v050)

## Unreleased

### Breaking Changes

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### New

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Improvements

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Fixes

- **Scaler**: remplement custom interceptor metrics ([#718](https://github.com/kedacore/http-add-on/issues/718))
- **Operator**: Remove ScaledObject `name` & `app` custom labels ([#717](https://github.com/kedacore/http-add-on/issues/717))
- **Interceptor**: fatal error: concurrent map iteration and map write ([#726](https://github.com/kedacore/http-add-on/issues/726))
- **Interceptor**: shutdown http servers gracefully on SIGINT and SIGTERM ([#731](https://github.com/kedacore/http-add-on/issues/731))
- **Scaler**: shutdown grpc server gracefully on SIGINT and SIGTERM ([#731](https://github.com/kedacore/http-add-on/issues/731))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

Previously announced deprecation(s):

- TODO

### Other

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

## v0.5.0

### Breaking Changes

None.

### New

- **Routing**: Add multi-host support to `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))
- **Routing**: Support path-based routing ([#338](https://github.com/kedacore/http-add-on/issues/338))
- **General**: Log incoming requests using the Combined Log Format ([#669](https://github.com/kedacore/http-add-on/pull/669))

### Improvements

- **General**: Automatically tag Docker image with commit SHA ([#567](https://github.com/kedacore/http-add-on/issues/567))
- **RBAC**: Introduce fine-grained permissions per component and reduce required permissions ([#612](https://github.com/kedacore/http-add-on/issues/612))
- **Operator**: Migrate project to Kubebuilder v3 ([#625](https://github.com/kedacore/http-add-on/issues/625))
- **Routing**: New routing table implementation that relies on the live state of HTTPScaledObjects on the K8s Cluster instead of a ConfigMap that is updated periodically ([#605](https://github.com/kedacore/http-add-on/issues/605))

### Fixes

- **General**: HTTPScaledObject is the owner of the underlying ScaledObject ([#703](https://github.com/kedacore/http-add-on/issues/703))
- **Routing**: Lookup host without port ([#608](https://github.com/kedacore/http-add-on/issues/608))
- **Controller**: Use kedav1alpha1.ScaledObject default values ([#607](https://github.com/kedacore/http-add-on/issues/607))
- **General**: Changes to HTTPScaledObjects now take effect ([#605](https://github.com/kedacore/http-add-on/issues/605))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

Previously announced deprecation(s):

- None.

### Other

- **General**: Use kubernetes e2e images for e2e test and samples ([#665](https://github.com/kedacore/http-add-on/issues/665))
- **e2e tests**: Use the same e2e system as in core ([#686](https://github.com/kedacore/http-add-on/pull/686))
