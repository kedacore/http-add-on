# Release Process

The process of releasing a new version of the KEDA HTTP Addon involves a few steps, detailed below.

>The process herein is largely automated but we recognize that there may be more that we can automate. If you find something that _can_ and _should_ be automated, and you believe that you know how, please [submit an issue](https://github.com/kedacore/http-add-on/issues/new?assignees=&labels=needs-discussion%2Cfeature-request&template=Feature_request.md) explaining how.

## 0: Current and new versions

Please go to the [releases page](https://github.com/kedacore/http-add-on/releases) and observe what the most recent release is. Specifically, note what the _tag_ of the release is. For example, if [version 0.1.0](https://github.com/kedacore/http-add-on/releases/tag/v0.1.0) is the latest release (it is as the time of this writing), the tag for that is `v0.1.0`.

To determine the new version, follow [SemVer guidelines](https://semver.org). Most releases will increment the PATCH or MINOR version number.

## 1: Submit a PR to the [Helm Charts Repository](https://github.com/kedacore/charts)

This step is not strictly necessary, since not every change to the HTTP Addon requires a Helm chart. In some cases, though, a Helm chart change will be necessary, like a change to the `HTTPScaledObject` or new configuration to one of the components (i.e. operator, interceptor, or scaler). If your release is one of those situations, do the following:

1. Submit a Pull Request (PR) to the [github.com/kedacore/charts](https://github.com/kedacore/charts) repository with your changes. Also ensure that you follow the [Shipping a new version](https://github.com/kedacore/charts/blob/master/CONTRIBUTING.md#shipping-a-new-version) guidelines in the charts documentation to complete the chart release.
   - Your chart changes must go into the [http-add-on](https://github.com/kedacore/charts/tree/master/http-add-on) directory. The release artifact will go into the [docs](https://github.com/kedacore/charts/tree/master/docs) directory.
   - Ensure that you add a link to the HTTP Addon repository and the new release number, so that PR reviewers are aware what the work relates to
2. Ensure that the Pull Request is reviewed and merged
    - This may require collaboration with reviewers and appropriate changes

## 2: Create a new GitHub release

[Create a new release](https://github.com/kedacore/http-add-on/releases/new) on the GitHub releases page, using your new release number.

The title of the release should be "Version 1.2.3", substituting `1.2.3` with the new version number, and the Git tag should be `v1.2.3`, again substituting `1.2.3` with your new version number.

The release description should be a short to medium length summary of what has changed since the last release. The following link will give you a list of commits made since the `v0.1.0` tag: https://github.com/kedacore/http-add-on/athens/compare/main...v0.1.0. Replace `v0.1.0` for your appropriate most recent last tag to get the commit list and base your release summary on that list.
