/*
Copyright 2024 The Kubernetes resource-state-metrics Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// probe defines behaviours for a health-check probe.
type probe interface {

	// getSource returns the originating server for the probe.
	getSource() string

	// getAsString returns the string representation of the probe.
	getAsString() string

	// Probe knows how to handle a health probe.
	probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler
}

// healthz implements the probe interface.
type healthz struct {

	// source is the originating server for the probe.
	source string

	// asString is the string representation of the probe.
	asString string
}

// newHealthz returns a new healthz probe.
func newHealthz(source string) probe {
	return healthz{
		source:   source,
		asString: "/healthz",
	}
}

// source returns the originating server for the probe.
func (h healthz) getSource() string {
	return h.source
}

// asString returns the string representation of the probe.
func (h healthz) getAsString() string {
	return h.asString
}

// Probe returns a healthz probe.
func (h healthz) probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return genericProbe(ctx, h, logger, client)
}

// livez implements the probe interface.
type livez struct {

	// source is the originating server for the probe.
	source string

	// asString is the string representation of the probe.
	asString string
}

// newLivez returns a new livez probe.
func newLivez(source string) probe {
	return livez{
		source:   source,
		asString: "/livez",
	}
}

// getSource returns the originating server for the probe.
func (l livez) getSource() string {
	return l.source
}

// getAsString returns the string representation of the probe.
func (l livez) getAsString() string {
	return l.asString
}

// Probe returns a livez probe.
func (l livez) probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return genericProbe(ctx, l, logger, client)
}

// readyz implements the probe interface.
type readyz struct {

	// source is the originating server for the probe.
	source string

	// asString is the string representation of the probe.
	asString string
}

// newReadyz returns a new readyz probe.
func newReadyz(source string) probe {
	return readyz{
		source:   source,
		asString: "/readyz",
	}
}

// getSource returns the originating server for the probe.
func (r readyz) getSource() string {
	return r.source
}

// getAsString returns the string representation of the probe.
func (r readyz) getAsString() string {
	return r.asString
}

// Probe returns a readyz probe.
func (r readyz) probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return genericProbe(ctx, r, logger, client)
}

// genericProbe returns an http.Handler that delegates probes to the Kubernetes API.
func genericProbe(ctx context.Context, p probe, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		got := client.CoreV1().RESTClient().Get().AbsPath(p.getAsString()).Do(ctx)
		if got.Error() != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			n, err := w.Write([]byte(http.StatusText(http.StatusServiceUnavailable)))
			if err != nil {
				logger.Error(err, fmt.Sprintf("error writing response after %d bytes", n), "probeType", p.getAsString(), "source", p.getSource())
			}

			return
		}

		w.WriteHeader(http.StatusOK)
		n, err := w.Write([]byte(http.StatusText(http.StatusOK)))
		if err != nil {
			logger.Error(err, fmt.Sprintf("error writing response after %d bytes", n), "probeType", p.getAsString(), "source", p.getSource())

			return
		}
	})
}
