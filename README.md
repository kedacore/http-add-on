<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [HTTP Autoscaling Made Simple](#http-autoscaling-made-simple)
- [Adopters - Become a listed KEDA user!](#adopters---become-a-listed-keda-user)
- [Walkthrough](#walkthrough)
- [Design](#design)
- [Installation](#installation)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Code of Conduct](#code-of-conduct)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

<p align="center"><img src="https://github.com/kedacore/keda/raw/main/images/logos/keda-word-colour.png" width="300"/></p>

<p style="font-size: 25px" align="center"><b>Kubernetes-based Event Driven Autoscaling - HTTP Add-on</b></p>
<p style="font-size: 25px" align="center">

The KEDA HTTP Add-on allows Kubernetes users to automatically scale their HTTP servers up and down (including to/from zero) based on incoming HTTP traffic. Please see our [use cases document](./docs/use_cases.md) to learn more about how and why you would use this project.

| 🚧 **Project status: beta** 🚧|
|---------------------------------------------|
| :loudspeaker: **KEDA is actively relying on community contributions to help grow & maintain the add-on. The KEDA maintainers are assisting the community to evolve the add-on but not directly responsible for it.** Feel free to [open a new discussion](https://github.com/kedacore/http-add-on/discussions/new/choose) in case of questions.<br/><br/>⚠ The HTTP Add-on currently is in [beta](https://github.com/kedacore/http-add-on/releases/latest). We can't yet recommend it for production usage because we are still developing and testing it. It may have "rough edges" including missing documentation, bugs and other issues. It is currently provided as-is without support.<br/><br/>:bulb: For production-ready needs, you can consider using the [Kedify HTTP Scaler](https://kedify.io/scalers/http), a commercial alternative offering robust and reliable scaling for KEDA. |

## HTTP Autoscaling Made Simple

[KEDA](https://github.com/kedacore/keda) provides a reliable and well tested solution to scaling your workloads based on external events. The project supports a wide variety of [scalers](https://keda.sh/docs/latest/scalers/) - sources of these events, in other words. These scalers are systems that produce precisely measurable events via an API.

KEDA does not, however, include an HTTP-based scaler out of the box for several reasons:

- The concept of an HTTP "event" is not well defined.
- There's no out-of-the-box single system that can provide an API to measure the current number of incoming HTTP events or requests.
- The infrastructure required to achieve these measurements is more complex and, in some cases, needs to be integrated into the HTTP routing system in the cluster (e.g. the ingress controller).

For these reasons, the KEDA core project has purposely not built generic HTTP-based scaling into the core.

This project, often called KEDA-HTTP, exists to provide that scaling. It is composed of simple, isolated components and includes an opinionated way to put them together.

## Adopters - Become a listed KEDA user!

We are always happy to start list users who run KEDA's HTTP Add-on in production or are evaluating it, learn more about it [here](ADOPTERS.md).

We welcome pull requests to list new adopters.

## Walkthrough

Although this is currently a **beta release** project, we have prepared a walkthrough document with instructions on getting started for basic usage.

See that document at [docs/walkthrough.md](./docs/walkthrough.md)

## Design

The HTTP Add-on is composed of multiple mostly independent components. This design was chosen to allow for highly
customizable installations while allowing us to ship reasonable defaults.

- We have written a complete design document. Please see it at [docs/design.md](./docs/design.md).
- For more context on the design, please see our [scope document](./docs/scope.md).
- If you have further questions about the project, please see our [FAQ document](./docs/faq.md).

## Installation

Please see the [complete installation instructions](./docs/install.md).

## Roadmap
We use GitHub issues to build our backlog, a complete overview of all open items and our planning.

Learn more about our [roadmap](ROADMAP.md).

## Contributing

This project follows the KEDA contributing guidelines, which are outlined in [CONTRIBUTING.md](https://github.com/kedacore/.github/blob/main/CONTRIBUTING.md).

If you would like to contribute code to this project, please see [docs/developing.md](./docs/developing.md).

---
We are a Cloud Native Computing Foundation (CNCF) graduated project.
<p align="center"><img src="https://raw.githubusercontent.com/kedacore/keda/main/images/logo-cncf.svg" height="75px"></p>

## Code of Conduct

Please refer to the organization-wide [Code of Conduct document](https://github.com/kedacore/.github/blob/main/CODE_OF_CONDUCT.md).
