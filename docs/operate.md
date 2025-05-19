# Configuring metrics for the KEDA HTTP Add-on interceptor proxy

### Exportable metrics:
* **Pending request count** - the number of pending requests for a given host.
* **Total request count** - the total number of requests for a given host with method, path and response code attributes.

There are currently 2 supported methods for exposing metrics from the interceptor proxy service - via a Prometheus compatible metrics endpoint or by pushing metrics to a OTEL HTTP collector.

### Configuring the Prometheus compatible metrics endpoint
When configured, the interceptor proxy can expose metrics on a Prometheus compatible endpoint.

This endpoint can be enabled by setting the `OTEL_PROM_EXPORTER_ENABLED` environment variable to `true` on the interceptor deployment (`true` by default) and by setting `OTEL_PROM_EXPORTER_PORT` to an unused port for the endpoint to be made avaialble on (`2223` by default).

### Configuring the OTEL HTTP exporter
When configured, the interceptor proxy can export metrics to a OTEL HTTP collector.

The OTEL exporter can be enabled by setting the `OTEL_EXPORTER_OTLP_METRICS_ENABLED` environment variable to `true` on the interceptor deployment (`false` by default). When enabled the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable must also be configured so the exporter knows what collector to send the metrics to (e.g. http://opentelemetry-collector.open-telemetry-system:4318).

If you need to provide any headers such as authentication details in order to utilise your OTEL collector you can add them into the `OTEL_EXPORTER_OTLP_HEADERS` environment variable. The frequency at which the metrics are exported can be configured by setting `OTEL_METRIC_EXPORT_INTERVAL` to the number of seconds you require between each export interval (`30` by default).

# Configuring TLS for the KEDA HTTP Add-on interceptor proxy

The interceptor proxy has the ability to run both a HTTP and HTTPS server simultaneously to allow you to scale workloads that use either protocol. By default, the interceptor proxy will only serve over HTTP, but this behavior can be changed by configuring the appropriate environment variables on the deployment.

The TLS server can be enabled by setting the environment variable `KEDA_HTTP_PROXY_TLS_ENABLED` to `true` on the interceptor deployment (`false` by default). The TLS server will start on port `8443` by default, but this can be configured by setting `KEDA_HTTP_PROXY_TLS_PORT` to your desired port number. The TLS server will require valid TLS certificates to start, the path to the certificates can be configured via the `KEDA_HTTP_PROXY_TLS_CERT_PATH` and `KEDA_HTTP_PROXY_TLS_KEY_PATH` environment variables (`/certs/tls.crt` and `/certs/tls.key` by default).

For setting multiple TLS certs, set `KEDA_HTTP_PROXY_TLS_CERT_STORE_PATHS` with comma-separated list of directories that will be recursively searched for any valid cert/key pairs. Currently, two naming patterns are supported
* `XYZ.crt` + `XYZ.key` - this is a convention when using Kubernetes Secrets of type tls
* `XYZ.pem` + `XYZ-key.pem`

The matching between certs and requests is performed during the TLS ClientHelo message, where the SNI service name is compared to SANs provided in each cert and the first matching cert will be used for the rest of the TLS handshake.
# Configuring tracing for the KEDA HTTP Add-on interceptor proxy

### Supported Exporters:
* **console** - The console exporter is useful for development and debugging tasks, and is the simplest to set up.
* **http/protobuf** - To send trace data to an OTLP endpoint (like the collector or Jaeger >= v1.35.0) you’ll want to configure an OTLP exporter that sends to your endpoint.
* * **grpc** - To configure exporter to send trace data over gRPC connection to an OTLP endpoint (like the collector or Jaeger >= v1.35.0) you’ll want to configure an OTLP exporter that sends to your endpoint.

### Configuring tracing with console exporter

To enable tracing with the console exporter, the `OTEL_EXPORTER_OTLP_TRACES_ENABLED` environment variable should be set to `true` on the interceptor deployment. (`false` by default).
Secondly set `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` to `console` (`console` by default). Other protocols include (`http/protobuf` and `grpc`).
Finally set `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` to `"http://localhost:4318/v1/traces"` (`"http://localhost:4318/v1/traces"` by default).


### Configuring tracing with OTLP exporter
When configured, the interceptor proxy can export metrics to a OTEL HTTP collector.

To enable tracing with otlp exporter, the `OTEL_EXPORTER_OTLP_TRACES_ENABLED` environment variable should be set to `true` on the interceptor deployment. (`false` by default).
Secondly set `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` to `otlphttp` (`console` by default). Other protocols include (`http/protobuf` and `grpc`)
Finally set `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` to the collector to send the traces to (e.g. http://opentelemetry-collector.open-telemetry-system:4318/v1/traces) (`"http://localhost:4318/v1/traces"` by default).
NOTE: full path is required to be set including <scheme><url><port><path>


Optional variables
`OTEL_EXPORTER_OTLP_HEADERS` - To pass any extra headers to the spans to utilise your OTEL collector e.g. authentication details (`"key1=value1,key2=value2"`)
`OTEL_EXPORTER_OTLP_TRACES_INSECURE` - To send traces to the tracing via HTTP rather than HTTPS (`false` by default)
`OTEL_EXPORTER_OTLP_TRACES_TIMEOUT` - The batcher timeout in seconds to send batch of data points (`5` by default)
