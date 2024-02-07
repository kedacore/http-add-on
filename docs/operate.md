# Configuring metrics for the KEDA HTTP Add-on interceptor proxy

### Exportable metrics:
* **Pending request count** - the number of pending requests for a given host.
* **Total request count** - the total number of requests for a given host with method, path and response code attributes.

There are currently 2 supported methods for exposing metrics from the interceptor proxy service - via a Prometheus compatible metrics endpoint or by pushing metrics to a OTEL HTTP collector.

### Configuring the Prometheus compatible metrics endpoint
When configured, the interceptor proxy can expose metrics on a Prometheus compatible endpoint.

This endpoint can be enabled by setting the `KEDA_HTTP_OTEL_PROM_EXPORTER_ENABLED` environment variable to `true` on the interceptor deployment (`true` by default) and by setting `KEDA_HTTP_OTEL_PROM_EXPORTER_PORT` to an unused port for the endpoint to be made avaialble on (`2223` by default).

### Configuring the OTEL HTTP exporter
When configured, the interceptor proxy can export metrics to a OTEL HTTP collector.

The OTEL exporter can be enabled by setting the `KEDA_HTTP_OTEL_HTTP_EXPORTER_ENABLED` environment variable to `true` on the interceptor deployment (`false` by default). When enabled the `KEDA_HTTP_OTEL_HTTP_COLLECTOR_ENDPOINT` environment variable must also be configured so the exporter knows what collector to send the metrics to (e.g. opentelemetry-collector.open-telemetry-system:4318).

If the collector is exposed on a unsecured endpoint then you can set the `KEDA_HTTP_OTEL_HTTP_COLLECTOR_INSECURE` environment variable to `true` (`false` by default) which will disable client security on the exporter.

If you need to provide any headers such as authentication details in order to utilise your OTEL collector you can add them into the `KEDA_HTTP_OTEL_HTTP_HEADERS` environment variable. The frequency at which the metrics are exported can be configured by setting `KEDA_HTTP_OTEL_METRIC_EXPORT_INTERVAL` to the number of seconds you require between each export interval (`30` by default).
