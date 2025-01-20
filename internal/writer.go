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
	"fmt"
	"io"
)

// metricsWriter knows how to write metrics for the groups of metric families present in the group of stores it holds
// to an io.Writer.
type metricsWriter struct {
	stores []*StoreType
}

// newMetricsWriter returns a new metricsWriter.
func newMetricsWriter(stores ...*StoreType) *metricsWriter {
	return &metricsWriter{
		stores: stores,
	}
}

// writeAllTo writes out metrics from the underlying stores to the given writer per resource. It writes metrics so that
// the ones with the same name are grouped together when written out, and guarantees an exposition format that is safe
// to be ingested by Prometheus.
func (m metricsWriter) writeAllTo(writer io.Writer) error {
	if len(m.stores) == 0 {
		return nil
	}
	for _, s := range m.stores {
		s.mutex.RLock()
		defer s.mutex.RUnlock()
	}
	for j := range len(m.stores) {
		for i, header := range m.stores[j].headers {
			if header != "" && header != "\n" {
				header += "\n"
			}
			n, err := writer.Write([]byte(header))
			if err != nil {
				return fmt.Errorf("error writing Help text (%s) after %d bytes: %w", header, n, err)
			}
			for _, metricFamilies := range m.stores[j].metrics {
				if i >= len(metricFamilies) {
					continue
				}
				n, err = writer.Write([]byte(metricFamilies[i]))
				if err != nil {
					return fmt.Errorf("error writing metric family after %d bytes: %w", n, err)
				}
			}
		}
	}

	return nil
}
