# Changelog

<!--
    New changelog entries must be inline with our changelog guidelines.
    Please refer to https://github.com/kedacore/keda/blob/main/CONTRIBUTING.md#Changelog to learn more.
-->

This changelog keeps track of work items that have been completed and are ready to be shipped in the next release.

## History

- [Unreleased](#unreleased)
- [v0.12.2](#v0122)
- [v0.12.1](#v0121)
- [v0.12.0](#v0120)
- [v0.11.1](#v0111)
- [v0.11.0](#v0110)
- [v0.10.0](#v0100)
- [v0.9.0](#v090)
- [v0.8.0](#v080)
- [v0.7.0](#v070)
- [v0.6.0](#v060)
- [v0.5.0](#v050)

## Unreleased

### Breaking Changes

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### New

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Improvements

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Fixes

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Deprecations

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))

### Other

- **General**: TODO ([#TODO](https://github.com/kedacore/http-add-on/issues/TODO))
- **Interceptor**: Remove unused `CurrentNamespace` field and `KEDA_HTTP_CURRENT_NAMESPACE` env var from interceptor config([#1484](https://github.com/kedacore/http-add-on/pull/1484))

## v0.12.2

### Improvements

- **Interceptor**: Increase default connection pool sizes for higher throughput ([#1469](https://github.com/kedacore/http-add-on/pull/1469))

### Fixes

- **Interceptor**: Fix health probes failing when HTTPScaledObject has no hosts configured ([#1447](https://github.com/kedacore/http-add-on/issues/1447))
- **Interceptor**: Rework dial retry to tolerate slow Service IP propagation and add `KEDA_HTTP_DIAL_RETRY_TIMEOUT` ([#1449](https://github.com/kedacore/http-add-on/issues/1449))
- **Interceptor**: Set endpoint informer resync default to 1s to suppress warning ([#1455](https://github.com/kedacore/http-add-on/issues/1455))

## v0.12.1

### Fixes

- **Interceptor**: Preserve `X-Forwarded-Proto` and `X-Forwarded-Host` headers from upstream proxies ([#1432](https://github.com/kedacore/http-add-on/issues/1432))

## v0.12.0

### Breaking Changes

- **Operator**: Migrate HTTPScaledObject status conditions to standard `metav1.Condition` type ([#1409](https://github.com/kedacore/http-add-on/issues/1409))

### New

- **General**: Add environment variables for leader election timing configuration ([#1365](https://github.com/kedacore/http-add-on/pull/1365))
- **General**: Add multi-level wildcard host matching for HTTPScaledObject routing ([#281](https://github.com/kedacore/http-add-on/issues/281))

### Improvements

- **General**: Make interceptor request logging optional ([#1375](https://github.com/kedacore/http-add-on/pull/1375))
- **Interceptor**: Add support for full-duplex HTTP1.1 ([#1386](https://github.com/kedacore/http-add-on/pull/1386))
- **Interceptor**: Route requests by HTTP header matching ([#1177](https://github.com/kedacore/http-add-on/issues/1177))
- **Interceptor**: Significant performance improvements ([#1403](https://github.com/kedacore/http-add-on/issues/1403))

### Fixes

- **Interceptor**: Decouple connection retry backoff from TCP dial timeout for faster cold starts ([#1385](https://github.com/kedacore/http-add-on/issues/1385))
- **Operator**: Fix conflict errors when updating HTTPScaledObject status ([#1402](https://github.com/kedacore/http-add-on/issues/1402))
- **Operator**: Prevent duplicate Ready conditions in HTTPScaledObject status by matching on Type instead of Reason ([#1390](https://github.com/kedacore/http-add-on/issues/1390))

### Deprecations

None

### Other

- **General**: Fix Makefile to support spaces in project base path ([#1392](https://github.com/kedacore/http-add-on/issues/1392))
- **General**: Get rid of kube-rbac-proxy ([#1369](https://github.com/kedacore/http-add-on/pull/1369))
- **General**: Update OpenTelemetry semconv version to v1.37.0 ([#1407](https://github.com/kedacore/http-add-on/issues/1407))
- **CI**: Replace stale bot with official GitHub Actions stale action ([#1398](https://github.com/kedacore/http-add-on/issues/1398))
- **CI**: Use GitHub-hosted ARM64 runners for e2e tests ([#1388](https://github.com/kedacore/http-add-on/issues/1388))
- **DevContainer**: Fix devcontainer build by updating deprecated Go tools ([#1347](https://github.com/kedacore/http-add-on/issues/1347))

## v0.11.1

### Improvements

- **General**: Updated k8s versions ([#1351](https://github.com/kedacore/http-add-on/pull/1351))
- **General**: Updated KEDA versions ([#1353](https://github.com/kedacore/http-add-on/pull/1353))

## v0.11.0

### Breaking Changes

- **General**: Migrate from deprecated `v1.Endpoints` to `discoveryv1.EndpointSlices` ([#1297](https://github.com/kedacore/http-add-on/issues/1297))

### New

- **General**: Add configurable tracing support to the interceptor proxy ([#1021](https://github.com/kedacore/http-add-on/pull/1021))
- **General**: Add failover service on cold-start ([#1280](https://github.com/kedacore/http-add-on/pull/1280))
- **General**: Add possibility to skip TLS verification for upstreams in interceptor ([#1307](https://github.com/kedacore/http-add-on/pull/1307))
- **General**: Allow using HSO and SO with different names ([#1293](https://github.com/kedacore/http-add-on/issues/1293))
- **General**: Support profiling for KEDA components ([#1308](https://github.com/kedacore/http-add-on/issues/1308))

### Improvements

- **General**: License compliance monitored with FOSSA ([#1423](https://github.com/kedacore/http-add-on/pull/1423))
- **Interceptor**: Support HTTPScaledObject scoped timeout ([#813](https://github.com/kedacore/http-add-on/issues/813))

### Other

- **Documentation**: Correct the service name used in the walkthrough documentation ([#1244](https://github.com/kedacore/http-add-on/pull/1244))

## v0.10.0

### New

- **General**: Fix infrastructure crashes when deleting ScaledObject while scaling ([#1245](https://github.com/kedacore/http-add-on/issues/1245))
- **General**: Fix kubectl active printcolumn ([#1211](https://github.com/kedacore/http-add-on/issues/1211))
- **General**: Support InitialCooldownPeriod for HTTPScaledObject ([#1213](https://github.com/kedacore/http-add-on/issues/1213))

### Other

- **Documentation**: Correct the service name used in the walkthrough documentation ([#1244](https://github.com/kedacore/http-add-on/pull/1244))

## v0.9.0

### Breaking Changes

- **General**: Drop support for deprecated field `spec.scaleTargetRef.deployment` ([#1061](https://github.com/kedacore/http-add-on/issues/1061))

### New

- **General**: Support portName in HTTPScaledObject service scaleTargetRef ([#1174](https://github.com/kedacore/http-add-on/issues/1174))
- **General**: Support setting multiple TLS certs for different domains on the interceptor proxy ([#1116](https://github.com/kedacore/http-add-on/issues/1116))
- **Interceptor**: Add support for for AWS ELB healthcheck probe ([#1198](https://github.com/kedacore/http-add-on/issues/1198))

### Fixes

- **General**: Align the interceptor metrics env var configuration with the OTEL spec ([#1031](https://github.com/kedacore/http-add-on/issues/1031))
- **General**: Include trailing 0 window buckets in RPS calculation ([#1075](https://github.com/kedacore/http-add-on/issues/1075))

### Other

- **General**: Sign images with Cosign ([#1062](https://github.com/kedacore/http-add-on/issues/1062))

## v0.8.0

### New

- **General**: Add configurable TLS on the wire support to the interceptor proxy ([#907](https://github.com/kedacore/http-add-on/issues/907))
- **General**: Add support for collecting metrics using a Prometheus compatible endpoint or by sending metrics to an OpenTelemetry's HTTP endpoint ([#910](https://github.com/kedacore/http-add-on/issues/910))
- **General**: Propagate HTTPScaledObject labels and annotations to ScaledObject ([#840](https://github.com/kedacore/http-add-on/issues/840))
- **General**: Provide support for allowing HTTP scaler to work alongside other core KEDA scalers ([#489](https://github.com/kedacore/http-add-on/issues/489))
- **General**: Support aggregation windows ([#882](https://github.com/kedacore/http-add-on/issues/882))

### Fixes

- **General**: Ensure operator is aware about changes on underlying ScaledObject ([#900](https://github.com/kedacore/http-add-on/issues/900))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

- **General**: Deprecated `targetPendingRequests` in favor of `spec.scalingMetric.*.targetValue` ([#959](https://github.com/kedacore/http-add-on/discussions/959))

### Other

- **General**: Align with the new format of Ingress in the example demo ([#979](https://github.com/kedacore/http-add-on/pull/979))
- **General**: Unify loggers ([#958](https://github.com/kedacore/http-add-on/issues/958))

## v0.7.0

### Breaking Changes

- **General**: `host` field has been removed in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552)|[#888](https://github.com/kedacore/http-add-on/pull/888))

### New

- **General**: Support any resource which implements `/scale` subresource ([#438](https://github.com/kedacore/http-add-on/issues/438))

### Improvements

- **General**: Improve Scaler reliability adding probes and 3 replicas ([#870](https://github.com/kedacore/http-add-on/issues/870))

### Fixes

- **General**: Add new user agent probe ([#862](https://github.com/kedacore/http-add-on/issues/862))
- **General**: Fix external scaler getting into bad state when retrieving queue lengths fails. ([#870](https://github.com/kedacore/http-add-on/issues/870))
- **General**: Increase ScaledObject polling interval to 15 seconds ([#799](https://github.com/kedacore/http-add-on/issues/799))
- **General**: Set forward request RawPath to original request RawPath ([#864](https://github.com/kedacore/http-add-on/issues/864))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: Deprecated `KEDA_HTTP_DEPLOYMENT_CACHE_POLLING_INTERVAL_MS` in favor of `KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS` ([#438](https://github.com/kedacore/http-add-on/issues/438))

### Other

- **General**: Bump golang version ([#853](https://github.com/kedacore/http-add-on/pull/853))

## v0.6.0

### New

- **General**: Add manifests to deploy the Add-on ([#716](https://github.com/kedacore/http-add-on/issues/716))

### Improvements

- **Scaler**: Decrease memory usage by allowing increasing stream interval configuration ([#745](https://github.com/kedacore/http-add-on/pull/745))

### Fixes

- **Interceptor**: Add support for streaming responses ([#743](https://github.com/kedacore/http-add-on/issues/743))
- **Interceptor**: Fatal error: concurrent map iteration and map write ([#726](https://github.com/kedacore/http-add-on/issues/726))
- **Interceptor**: Keep original Host in the Host header ([#331](https://github.com/kedacore/http-add-on/issues/331))
- **Interceptor**: Provide graceful shutdown for http servers on SIGINT and SIGTERM ([#731](https://github.com/kedacore/http-add-on/issues/731))
- **Operator**: Remove ScaledObject `name` & `app` custom labels ([#717](https://github.com/kedacore/http-add-on/issues/717))
- **Scaler**: Provide graceful shutdown for grpc server on SIGINT and SIGTERM ([#731](https://github.com/kedacore/http-add-on/issues/731))
- **Scaler**: Reimplement custom interceptor metrics ([#718](https://github.com/kedacore/http-add-on/issues/718))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

### Other

- **General**: Adding a changelog validating script to check for formatting and order ([#761](https://github.com/kedacore/http-add-on/pull/761))
- **General**: Skip not required CI checks on PRs on new commits ([#801](https://github.com/kedacore/http-add-on/pull/801))

## v0.5.0

### Breaking Changes

None.

### New

- **General**: Log incoming requests using the Combined Log Format ([#669](https://github.com/kedacore/http-add-on/pull/669))
- **Routing**: Add multi-host support to `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))
- **Routing**: Support path-based routing ([#338](https://github.com/kedacore/http-add-on/issues/338))

### Improvements

- **General**: Automatically tag Docker image with commit SHA ([#567](https://github.com/kedacore/http-add-on/issues/567))
- **Operator**: Migrate project to Kubebuilder v3 ([#625](https://github.com/kedacore/http-add-on/issues/625))
- **RBAC**: Introduce fine-grained permissions per component and reduce required permissions ([#612](https://github.com/kedacore/http-add-on/issues/612))
- **Routing**: New routing table implementation that relies on the live state of HTTPScaledObjects on the K8s Cluster instead of a ConfigMap that is updated periodically ([#605](https://github.com/kedacore/http-add-on/issues/605))

### Fixes

- **General**: Changes to HTTPScaledObjects now take effect ([#605](https://github.com/kedacore/http-add-on/issues/605))
- **General**: HTTPScaledObject is the owner of the underlying ScaledObject ([#703](https://github.com/kedacore/http-add-on/issues/703))
- **Controller**: Use kedav1alpha1.ScaledObject default values ([#607](https://github.com/kedacore/http-add-on/issues/607))
- **Routing**: Lookup host without port ([#608](https://github.com/kedacore/http-add-on/issues/608))

### Deprecations

You can find all deprecations in [this overview](https://github.com/kedacore/http-add-on/labels/breaking-change) and [join the discussion here](https://github.com/kedacore/http-add-on/discussions/categories/deprecations).

New deprecation(s):

- **General**: `host` field deprecated in favor of `hosts` in `HTTPScaledObject` ([#552](https://github.com/kedacore/http-add-on/issues/552))

Previously announced deprecation(s):

- None.

### Other

- **General**: Use kubernetes e2e images for e2e test and samples ([#665](https://github.com/kedacore/http-add-on/issues/665))
- **e2e tests**: Use the same e2e system as in core ([#686](https://github.com/kedacore/http-add-on/pull/686))
