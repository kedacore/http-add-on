# Common Use Cases

This document includes several common scenarios in which this project may be deployed along with descriptions on why and how it could be deployed in each case.

## Current Containerized HTTP Application In The Cloud, Migrating to Kubernetes

In this use case, an application may be containerized running on a managed cloud platform that supports containers. Below is a non-exhaustive, alphabetically-ordered list of some examples:

- [Amazon ECS](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html)
- [Azure App Services](https://docs.microsoft.com/en-us/azure/app-service/quickstart-custom-container?pivots=container-linux)
- - [Digital Ocean App Platform](https://www.digitalocean.com/products/app-platform/)
- [Google App Engine Flexible Environment](https://cloud.google.com/appengine/docs/flexible/)

The platform may or may not be autoscaling.

Moving this application to Kubernetes may make sense for several reasons, but the pros and cons of that decision are out of scope of this document.

### How You'd Move This Application to KEDA-HTTP

If the application _is_ being moved to Kubernetes, you would follow these steps to get it autoscaling and routing with KEDA-HTTP:

- Create a `Deployment` and `Service`
- [Install](./install.md) the HTTP Add On
- Create a single `HTTPScaledObject` in the same namespace as the `Deployment` and `Service` you created

At that point, the operator will create the proper autoscaling and routing infrastructure behind the scenes and the application will be ready to scale.

## Current HTTP Server in Kubernetes

In this use case, an HTTP application is already running in Kubernetes, possibly (but not necessarily) already serving in production to the public internet.

In this case, the reasoning for adding the HTTP Add On would be clear - adding autoscaling based on incoming HTTP traffic. Turning this functionality on can be done transparently and without downtime. An administrator would follow the following two steps:

- [Install](./install.md) the add on. This step will have no effect on the running application.
- Create a new `HTTPScaledObject`. This step activates autoscaling for the `Deployment` that you specify and the application will immediately start scaling up and down based on incoming traffic.
