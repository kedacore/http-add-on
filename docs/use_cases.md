# Common Use Cases

This document includes several common scenarios in which this project may be deployed along with descriptions on why and how it could be deployed in each case.

## Current Containerized HTTP Application In The Cloud, Migrating to Kubernetes

In this use case, an application may be containerized running on a managed cloud platform that supports containers. Below is a non-exhaustive, non-ordered list of some examples:

- [Azure App Services](https://docs.microsoft.com/en-us/azure/app-service/quickstart-custom-container?pivots=container-linux)
- [Google App Engine Flexible Environment](https://cloud.google.com/appengine/docs/flexible/)
- [Amazon ECS](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html)
- [Digital Ocean App Platform](https://www.digitalocean.com/products/app-platform/)

The platform may or may not be autoscaling.

Moving this application to Kubernetes may make sense for several reasons, but the pros and cons of that decision are out of scope of this document.

If the application _is_ being moved to Kubernetes, though, a `Deployment`, `Service`, and several other resources will need to be created. Several other things may need to be changed, like CI/CD pipelines, ingress controllers, and so on. Once done, the application will successfully be running in production on Kubernetes.

To add intelligent routing and autoscaling to the infrastructure, the HTTP Add On will need to be [installed](./install.md) and a single `HTTPScaledObject` created in the same namespace as the application's `Deployment` is running.

At that point, the operator will create the proper autoscaling and routing infrastructure behind the scenes and the application will be ready to scale.

## Current HTTP Server in Kubernetes

In this use case, an HTTP application is already running in Kubernetes, possibly (but not necessarily) already serving in production to the public internet.

In this case, the reasoning for adding the HTTP Add On would be clear - adding autoscaling based on incoming HTTP traffic. Turning this functionality on can be done transparently and without downtime. An administrator would follow the following two steps:

- [Install](./install.md) the add on. This step will have no effect on the running application.
- Create a new `HTTPScaledObject`. This step activates autoscaling for the `Deployment` that you specify and the application will immediately start scaling up and down based on incoming traffic.
