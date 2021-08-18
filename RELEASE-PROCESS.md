# Release Process

The process of releasing a new version of the KEDA HTTP Addon involves a few steps, detailed below.

>The process herein is largely automated but we recognize that there may be more that we can automate. If you find something that _can_ and _should_ be automated, and you believe that you know how, please [submit an issue](https://github.com/kedacore/http-add-on/issues/new?assignees=&labels=needs-discussion%2Cfeature-request&template=Feature_request.md) explaining how.

## 0: Current and new versions

Please go to the [releases page](https://github.com/kedacore/http-add-on/releases) and observe what the most recent release is. Specifically, note what the _tag_ of the release is. For example, if [version 0.1.0](https://github.com/kedacore/http-add-on/releases/tag/v0.1.0) is the latest release (it is as the time of this writing), the tag for that is `v0.1.0`.

To determine the new version, follow [SemVer guidelines](https://semver.org). Most releases will increment the PATCH or MINOR version number.

## 1: Submit a PR to the [Helm Charts Repository](https://github.com/kedacore/charts)

The scope of the changes you'll need to make to the Helm chart vary from just changing the `appVersion` field in the [Chart.yaml file](https://github.com/kedacore/charts/blob/master/http-add-on/Chart.yaml) to changing the `HTTPScaledObject` CRD, adding new configuration, or even adding/changing components.

You must, at a minimum, change that `appVersion` field to the new version number, however. If you have chosen `1.2.3`, for example, the `appVersion` field should read:

```yaml
appVersion: 1.2.3
```

See the below steps for updating the Helm chart:

1. Submit a Pull Request (PR) to the [github.com/kedacore/charts](https://github.com/kedacore/charts) repository with your changes. Also ensure that you follow the [Shipping a new version](https://github.com/kedacore/charts/blob/master/CONTRIBUTING.md#shipping-a-new-version) guidelines in the charts documentation to complete the chart release.
   - Your chart changes must go into the [http-add-on](https://github.com/kedacore/charts/tree/master/http-add-on) directory. The release artifact will go into the [docs](https://github.com/kedacore/charts/tree/master/docs) directory.
   - Ensure that you add a link to the HTTP Addon repository and the new release number, so that PR reviewers are aware what the work relates to
2. Ensure that the Pull Request is reviewed and merged
    - This may require collaboration with reviewers and appropriate changes

## 2: Create a new GitHub release

[Create a new release](https://github.com/kedacore/http-add-on/releases/new) on the GitHub releases page, using your new release number.

The title of the release should be "Version 1.2.3", substituting `1.2.3` with the new version number, and the Git tag should be `v1.2.3`, again substituting `1.2.3` with your new version number.

The release description should be a short to medium length summary of what has changed since the last release. The following link will give you a list of commits made since the `v0.1.0` tag: [github.com/kedacore/http-add-on/compare/v0.1.0...main](https://github.com/kedacore/http-add-on/compare/v0.1.0...main). Replace `v0.1.0` for your appropriate most recent last tag to get the commit list and base your release summary on that list.

After you create the new release, automation in a GitHub action will build and deploy new container images.

## 3: Write a blog post on the documentation site (_optional_)

If you believe that your release is large enough to warrant a blog post on the [keda.sh/blog](https://keda.sh/blog/) site, please go to [github.com/kedacore/keda-docs](https://github.com/kedacore/keda-docs) and submit a new PR with a blog article about the release. Include in the article a longer summary of changes and any important information about the new functionality, bugfixes, or anything else appropriate. The post should go into the [content/blog](https://github.com/kedacore/keda-docs/tree/master/content/blog) directory.
