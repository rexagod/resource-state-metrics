/*
Copyright 2025 The Kubernetes resource-state-metrics Authors.

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
	server() string
	text() string
	probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler
}

// healthz implements the probe interface.
type healthz struct {
	source   string
	asString string
}

func newHealthz(source string) probe {
	return healthz{
		source:   source,
		asString: "/healthz",
	}
}

func (h healthz) server() string {
	return h.source
}

func (h healthz) text() string {
	return h.asString
}

func (h healthz) probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return genericProbe(ctx, h, logger, client)
}

// livez implements the probe interface.
type livez struct {
	source   string
	asString string
}

// newLivez returns a new livez probe.
func newLivez(source string) probe {
	return livez{
		source:   source,
		asString: "/livez",
	}
}

func (l livez) server() string {
	return l.source
}

func (l livez) text() string {
	return l.asString
}

func (l livez) probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return genericProbe(ctx, l, logger, client)
}

// readyz implements the probe interface.
type readyz struct {
	source   string
	asString string
}

// newReadyz returns a new readyz probe.
func newReadyz(source string) probe {
	return readyz{
		source:   source,
		asString: "/readyz",
	}
}

func (r readyz) server() string {
	return r.source
}

func (r readyz) text() string {
	return r.asString
}

func (r readyz) probe(ctx context.Context, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return genericProbe(ctx, r, logger, client)
}

// genericProbe returns an http.Handler that delegates probes to the Kubernetes API.
func genericProbe(ctx context.Context, p probe, logger klog.Logger, client kubernetes.Interface) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		got := client.CoreV1().RESTClient().Get().AbsPath(p.text()).Do(ctx)
		if got.Error() != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			n, err := w.Write([]byte(http.StatusText(http.StatusServiceUnavailable)))
			if err != nil {
				logger.Error(err, fmt.Sprintf("error writing response after %d bytes", n), "probeType", p.text(), "source", p.server())
			}

			return
		}
		w.WriteHeader(http.StatusOK)
		n, err := w.Write([]byte(http.StatusText(http.StatusOK)))
		if err != nil {
			logger.Error(err, fmt.Sprintf("error writing response after %d bytes", n), "probeType", p.text(), "source", p.server())

			return
		}
	})
}
