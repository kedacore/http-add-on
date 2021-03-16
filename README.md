<p align="center"><img src="https://github.com/kedacore/keda/raw/main/images/logos/keda-word-colour.png" width="300"/></p>

<p style="font-size: 25px" align="center"><b>Kubernetes-based Event Driven Autoscaling - HTTP Add-On</b></p>
<p style="font-size: 25px" align="center">

The KEDA HTTP Add On allows Kubernetes users to automatically scale their HTTP servers up and down (including to/from zero) based on incoming HTTP traffic. Please see our [use cases document](./docs/use_cases.md) to learn more about how and why you would use this project.

| 🚧 **Alpha - Not for production** 🚧|
|---------------------------------------------|
| ⚠ The HTTP add-on is in [experimental stage](https://github.com/kedacore/keda/issues/538) and not ready for production. <br /><br />It is provided as-is without support.

>This codebase moves very quickly. We can't currently guarantee that any part of it will work. Neither the complete feature set nor known issues may be fully documented. Similarly, issues filed against this project may not be responded to quickly or at all. **We will release and announce a beta release of this project**, and after we do that, we will document and respond to issues properly.

## Walkthrough

Although this is an **alpha release** project right now, we have prepared a walkthrough document that with instructions on getting started for basic usage.

See that document at [docs/walkthrough.md](./docs/walkthrough.md)

## Design

The HTTP add-on is composed of multiple mostly independent components. This design was chosen to allow for highly
customizable installations while allowing us to ship reasonable defaults.

- We have written a complete design document. Please see it at [docs/design.md](./docs/design.md).
- For more context on the design, please see our [scope document](./docs/scope.md).
- If you have further questions about the project, please see our [FAQ document](./docs/faq.md).

## Installation

Please see the [complete installation instructions](./docs/install.md).

## Contributing

Please see the [contributing documentation for all instructions](./docs/contributing.md).

---
We are a Cloud Native Computing Foundation (CNCF) sandbox project.
<p align="center"><img src="https://raw.githubusercontent.com/kedacore/keda/main/images/logo-cncf.svg" height="75px"></p>
