---
name: KEDA Release Tracker
about: Template to keep track of the progress for a new KEDA HTTP add-on release.
title: "Release: "
labels: governance,release-management
---

This issue template is used to track the rollout of a new KEDA HTTP add-on version.

For the full release process, we recommend reading [this document](https://github.com/kedacore/http-add-on/blob/main/RELEASE-PROCESS.md).

## Required items

- [ ] List items that are still open, but required for this release

## Timeline

We aim to release this release in the week of <week range, example March 27-31>.

## Progress

- [ ] Create the KEDA HTTP Add-on release
- [ ] Prepare & ship the Helm chart
- [ ] Create a new documentation version in [kedacore/keda-docs](https://github.com/kedacore/keda-docs) (see [RELEASE-PROCESS.md](https://github.com/kedacore/http-add-on/blob/main/RELEASE-PROCESS.md#4-create-a-new-documentation-version))
- [ ] Publish on Artifact Hub ([repo](https://github.com/kedacore/external-scalers))
- [ ] Provide update in Slack
