# Contributing to KEDA

Thanks for helping make KEDA better 😍.

There are many areas we can use contributions - ranging from code, documentation, feature proposals, issue triage, samples, and content creation.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
## Table of contents

- [Project governance](#project-governance)
- [Including Documentation Changes](#including-documentation-changes)
- [Development Environment Setup](#development-environment-setup)
- [Locally Build & Deploy KEDA HTTP Addon](#locally-build--deploy-keda-http-addon)
  - [Pre-requisite:](#pre-requisite)
  - [Building:](#building)
  - [Deploying:](#deploying)
  - [Load testing with k9s:](#load-testing-with-k9s)
- [Developer Certificate of Origin: Signing your work](#developer-certificate-of-origin-signing-your-work)
  - [Every commit needs to be signed](#every-commit-needs-to-be-signed)
  - [I didn't sign my commit, now what?!](#i-didnt-sign-my-commit-now-what)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Project governance

You can learn about the governance of KEDA [here](https://github.com/kedacore/governance).

## Including Documentation Changes

For any contribution you make that impacts the behavior or experience of the KEDA HTTP Add-on, please make sure you include updates to the documentation in the same pull request, and if appropriate, changes to [the KEDA main documentation repository](https://github.com/kedacore/keda-docs). Contributions that do not include documentation or samples will be rejected.

## Development Environment Setup

We have a comprehensive how-to document that details the setup and configuration of the development environment for this project.

Please find it at [docs/developing.md](./docs/developing.md).

## Locally Build & Deploy KEDA HTTP Addon

### Pre-requisite:

- A running Kubernetes cluster with KEDA installed.
- [Make](https://www.gnu.org/software/make/)
- [Helm](https://helm.sh/)
- [k9s](https://github.com/derailed/k9s) (_optional_)

### Building:

- Fork & clone the repo:
  ```bash
  $ git clone https://github.com/<your-username>/http-add-on.git
  ```
- Change into the repo directory:
  ```bash
  $ cd http-add-on
  ```
- Use Make to build with:
   ```bash
   $ make build         # build local binaries
   $ make docker-build  # build docker images of the components
   ```

### Deploying:

Custom HTTP Add-on as an image

- Make your changes in the code
- Build and publish images with your changes, remembering to update the information for registry of your choice:

```bash
IMAGE_REGISTRY=docker.io IMAGE_REPO=johndoe make docker-publish
```

> Note: If you need to build images to other architecture from your machine, you can use multi-arch building with `IMAGE_REGISTRY=docker.io IMAGE_REPO=johndo make publish-multiarch`.

There are local clusters with local registries provided, in such cases make sure to use and push your images to its local registry. In the case of MicroK8s, the address is `localhost:32000` and the  command would look like the following.

```bash
IMAGE_REGISTRY=localhost:32000 IMAGE_REPO=johndo make deploy
```
### Load testing with k9s:

K9s integrates Hey, a CLI tool to benchmark HTTP endpoints similar to AB bench. This preliminary feature currently supports benchmarking port-forwards and services. You can use this feature in load testing as follows:

- Install an application to scale, we use the provided sample -
  ```console
  $ helm install xkcd ./examples/xkcd -n ${NAMESPACE}
  ```
- You'll need to clone the repository to get access to this chart. If you have your own Deployment and Service installed, you can go right to creating an HTTPScaledObject. We use the provided sample HTTPScaledObject -
  ```
  $ kubectl apply -n $NAMESPACE -f examples/v0.10.0/httpscaledobject.yaml
  ```
- Testing Your Installation using k9s:
  ```
  (a) Enter k9s dashboard using command:  `k9s`

  (b) search for services using - “:service”

  (c) HTTP traffic needs to route through the Service that the add on has set up. Find interceptor proxy service i.e. ‘keda-add-ons-http-interceptor-proxy’ and port forward it using <SHIFT+F>

  (d) Search for the same port-forward in the list you get by command - “:pf”

  (e) Enter the port-forward and apply <CTRL+L> to start a benchmark

  (f) You can enter the port-forward to see the run stat details and performance.
  ```
>You can customize the benchmark in k9s also. It's explained well in [here](https://k9scli.io/topics/bench/).

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

```console
$ git commit -s -m 'This is my commit message'
```

Each Pull Request is checked whether or not commits in a Pull Request do contain a valid Signed-off-by line.

### I didn't sign my commit, now what?!

No worries - You can easily replay your changes, sign them and force push them!

```console
$ git checkout <branch-name>
$ git reset $(git merge-base main <branch-name>)
$ git add -A
$ git commit -sm "one commit on <branch-name>"
$ git push --force
```
