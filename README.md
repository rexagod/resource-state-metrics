# `crsm`: Custom Resource State Metrics

[![Go Report Card](https://goreportcard.com/badge/github.com/rexagod/crsm)](https://goreportcard.com/report/github.com/rexagod/crsm) [![Go Reference](https://pkg.go.dev/badge/github.com/rexagod/crsm.svg)](https://pkg.go.dev/github.com/rexagod/crsm)

## Summary

Custom Resource State Metrics (`crsm`) is a Kubernetes controller that builds on Kube-State-Metrics' Custom Resource State's ideology and generates metrics for custom resources based on the configuration specified in its managed resource, `CustomResourceStateMetricsResource`.

## Development

Start developing by following these steps:

- Set up dependencies with `make setup`.
- Test out your changes with `POD_NAMESPACE=<controller-namespace> make apply apply-testdata local`.
  - Telemetry metrics, by default, are exposed on `:9998/metrics`.
  - Resource metrics, by default, are exposed on `:9999/metrics`.
- Start a `pprof` interactive session with `make pprof`.

For more details, take a look at the [Makefile](Makefile) targets.

## Notes

- Metrics stability: There are no metrics [stability](https://kubernetes.io/blog/2021/04/23/kubernetes-release-1.21-metrics-stability-ga/) guarantees, as the metrics are user-generated.
- Garbage in, garbage out: Invalid configurations will generate invalid metrics. The exception to this being that certain checks that ensure metric structure are still present (for e.g., `value` should be a `float64`).

## TODO

In the order of priority:

- [ ] CEL expressions for metric generation.
- [ ] Conformance tests covering Kube-State-Metrics' Custom Resource State's test cases.
- [ ] E2E tests covering the controller's functionality.
- [ ] [Graduate to ALPHA](https://github.com/kubernetes/enhancements/issues/4785).
- [ ] gRPC server for metrics generation.

###### [License](./LICENSE)
