# Changelog

<!--
    New changelog entries must be inline with our changelog guidelines.
    Please refer to https://github.com/kedacore/keda/blob/main/CONTRIBUTING.md#Changelog to learn more.
-->

This changelog keeps track of work items that have been completed and are ready to be shipped in the next release.

## History

- [Unreleased](#unreleased)

## Unreleased

### Breaking Changes

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### New

- **General**: Add multi-host support to `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

### Improvements

- **General**: Automatically tag Docker image with commit SHA ([#567](https://github.com/kedacore/http-add-on/issues/567))
- **RBAC**: Introduce fine-grained permissions per component and reduce required permissions ([#612](https://github.com/kedacore/http-add-on/issues/612))
- **Operator**: Migrate project to Kubebuilder v3 ([#625](https://github.com/kedacore/http-add-on/issues/625))

### Fixes

- **Routing**: Lookup host without port ([#608](https://github.com/kedacore/http-add-on/issues/608))
- **Controller**: Use kedav1alpha1.ScaledObject default values ([#607](https://github.com/kedacore/http-add-on/issues/607))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

Previously announced deprecation(s):

- TODO

### Other

- **e2e tests**: Use the same e2e system as in core ([#686]https://github.com/kedacore/http-add-on/pull/686)
