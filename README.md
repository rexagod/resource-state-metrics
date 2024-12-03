# `resource-state-metrics`

[![CI](https://github.com/rexagod/resource-state-metrics/actions/workflows/continuous-integration.yaml/badge.svg)](https://github.com/rexagod/resource-state-metrics/actions/workflows/continuous-integration.yaml) [![Go Report Card](https://goreportcard.com/badge/github.com/rexagod/resource-state-metrics)](https://goreportcard.com/report/github.com/rexagod/resource-state-metrics) [![Go Reference](https://pkg.go.dev/badge/github.com/rexagod/resource-state-metrics.svg)](https://pkg.go.dev/github.com/rexagod/resource-state-metrics)

## Summary

`resource-state-metrics` is a Kubernetes controller that builds on Kube-State-Metrics' Custom Resource State's ideology and generates metrics for custom resources based on the configuration specified in its managed resource, `ResourceMetricsMonitor`.

The project's [conformance benchmarking](./tests/bench/bench.sh) shows 3x faster RTT for `resource-state-metrics` as compared to Kube-State-Metrics' Custom Resource Definition Metrics ([ea5826a](https://github.com/kubernetes/kube-state-metrics/commit/ea5826a92cde206fc6784d2cb6b7c2548d2b2290)) feature-set:

```
Thu Nov 21 05:06:09 IST 2024
[RESOURCESTATEMETRICS]
BUILD:	1059ms
RTT:	1107ms
[CUSTOMRESOURCESTATE]
BUILD:	1116ms
RTT:	3196ms
```

## Development

Start developing by following these steps:

- Set up dependencies with `make setup`.
- Test out your changes with `make apply apply-testdata local`.
  - Telemetry metrics, by default, are exposed on `:9998/metrics`.
  - Resource metrics, by default, are exposed on `:9999/metrics`.
- Start a `pprof` interactive session with `make pprof`.

For more details, take a look at the [Makefile](Makefile) targets.

## Notes

- Garbage in, garbage out: Invalid configurations will generate invalid metrics. The exception to this being that certain checks that ensure metric structure are still present (for e.g., `value` should be a `float64`).
- Library support: The module is **never** intended to be used as a library, and as such, does not export any functions or types, with `pkg/` being an exception (for managed types and such).
- Metrics stability: There are no metrics [stability](https://kubernetes.io/blog/2021/04/23/kubernetes-release-1.21-metrics-stability-ga/) guarantees, as the metrics are user-generated.
- No middle-ware: The configuration is `unmarshal`led into a set of stores that the codebase directly operates on. There is no middle-ware that processes the configuration before it is used, in order to avoid unnecessary complexity. However, the expression(s) within the `value` and `labelValues` may need to be evaluated before being used, and as such, are exceptions.
- The managed resource, `ResourceMetricsMonitor` is namespace-scoped, but, to keep in accordance with KSM's `CustomResourceState`, which allows for collecting metrics from cluster-wide resources, it is possible to omit any `field` or `label` selectors to achieve that result. Similarly, to isolate metrics between namespaces (or teams), the selectors may be levied, and a utility such as [`prom-label-proxy`](https://github.com/prometheus-community/prom-label-proxy) to enforce selective namespace(s) or custom label(s).
  - Enforce namespaced-collection behind a flag (for cluster admins)?

## TODO

In the order of priority:

- [X] CEL expressions for metric generation (or [*unstructured.Unstructured](https://github.com/kubernetes/apimachinery/issues/181), if that suffices).
- [X] Conformance test(s) for Kube-State-Metrics' [Custom Resource State API](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md#multiple-metricskitchen-sink).
- [X] Benchmark(s) for Kube-State-Metrics' [Custom Resource State API](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/extend/customresourcestate-metrics.md#multiple-metricskitchen-sink).
- [X] E2E tests covering the controller's basic functionality.
- [X] `s/CRSM/CRDMetrics`.
- [X] [Draft out a KEP](https://github.com/kubernetes/enhancements/issues/4785).
- [X] `s/CRDMetrics/ResourceStateMetrics`.
- [X] Make `ResourceMetricsMonitor` namespaced-scope. This allows for:
  - [X] per-namespace configuration (separate configurations between teams), and,
  - [ ] garbage collection (without finalizers), since currently the namespace-scoped deployment manages its cluster-scoped resources.
- [ ] Meta-metrics for metric generation failures. Also, traces?
- [ ] Dynamic admission control for `ResourceMetricsMonitor` CRD.

###### [License](./LICENSE)
