# Contributing to KEDA

Thanks for helping make KEDA better 😍.

There are many areas we can use contributions - ranging from code, documentation, feature proposals, issue triage, samples, and content creation.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of contents

- [Contributing to KEDA](#contributing-to-keda)
  - [Table of contents](#table-of-contents)
  - [Project governance](#project-governance)
  - [Including Documentation Changes](#including-documentation-changes)
  - [Development Environment Setup](#development-environment-setup)
  - [Developer Certificate of Origin: Signing your work](#developer-certificate-of-origin-signing-your-work)
    - [Every commit needs to be signed](#every-commit-needs-to-be-signed)
    - [I didn't sign my commit, now what?!](#i-didnt-sign-my-commit-now-what)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Project governance

You can learn about the governance of KEDA [here](https://github.com/kedacore/governance).

## Including Documentation Changes

For any contribution you make that impacts the behavior or experience of the KEDA HTTP Addon, please make sure you include updates to the documentation in the same pull request, and if appropriate, changes to [the KEDA main documentation repository](https://github.com/kedacore/keda-docs). Contributions that do not include documentation or samples will be rejected.

## Development Environment Setup

We have a comprehensive how-to document that details the setup and configuration of the development environment for this project.

Please find it at [docs/developing.md](./docs/developing.md).

## Developer Certificate of Origin: Signing your work

### Every commit needs to be signed

The Developer Certificate of Origin (DCO) is a lightweight way for contributors to certify that they wrote or otherwise have the right to submit the code they are contributing to the project. Here is the full text of the DCO, reformatted for readability:

```
By making a contribution to this project, I certify that:

    (a) The contribution was created in whole or in part by me and I have the right to submit it under the open source license indicated in the file; or

    (b) The contribution is based upon previous work that, to the best of my knowledge, is covered under an appropriate open source license and I have the right under that license to submit that work with modifications, whether created in whole or in part by me, under the same open source license (unless I am permitted to submit under a different license), as indicated in the file; or

    (c) The contribution was provided directly to me by some other person who certified (a), (b) or (c) and I have not modified it.

    (d) I understand and agree that this project and the contribution are public and that a record of the contribution (including all personal information I submit with it, including my sign-off) is maintained indefinitely and may be redistributed consistent with this project or the open source license(s) involved.
```

Contributors sign-off that they adhere to these requirements by adding a `Signed-off-by` line to commit messages.

```
This is my commit message

Signed-off-by: Random J Developer <random@developer.example.org>
```

Git even has a `-s` command line option to append this automatically to your commit message:

```
$ git commit -s -m 'This is my commit message'
```

Each Pull Request is checked whether or not commits in a Pull Request do contain a valid Signed-off-by line.

### I didn't sign my commit, now what?!

No worries - You can easily replay your changes, sign them and force push them!

```
git checkout <branch-name>
git reset $(git merge-base main <branch-name>)
git add -A
git commit -sm "one commit on <branch-name>"
git push --force
```
