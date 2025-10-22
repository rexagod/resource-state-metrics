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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

const (
	// metricTypeGauge represents the type of metric. This is pinned to `gauge` to avoid ingestion issues with different backends
	// (Prometheus primarily) that may not recognize all metrics under the OpenMetrics spec. This also helps upkeep a more
	// consistent configuration. Refer https://github.com/kubernetes/kube-state-metrics/pull/2270 for more details.
	metricTypeGauge = "gauge"
	// In convention with kube-state-metrics, we prefix all metrics with `kube_customresource_` to explicitly denote
	// that these are custom resource user-generated metrics (and have no stability).
	kubeCustomResourcePrefix = "kube_customresource_"
)

// FamilyType represents a metric family (a group of metrics with the same name).
type FamilyType struct {
	logger     klog.Logger
	Name       string        `yaml:"name"`
	Help       string        `yaml:"help"`
	Metrics    []*MetricType `yaml:"metrics"`
	AddonStubs []string      `yaml:"addonStubs,omitempty"` // merge with stubs
}

// buildMetrics returns the given family in its byte representation.
func (f *FamilyType) buildMetrics(unstructured *unstructured.Unstructured) string {
	logger := f.logger.WithValues("family", f.Name)
	familyRawBuilder := strings.Builder{}

	for _, metric := range f.Metrics {
		metricRawBuilder := strings.Builder{}

		// Inherit family-level addon stubs into each metric stub, so we can eventually merge the two.
		inheritMetricAttributes(f, metric)

		samples := metric.resolve(logger, unstructured)
		for _, sample := range samples {
			err := writeMetricSamples(&metricRawBuilder, f.Name, unstructured, sample.LabelKeys, sample.LabelValues, sample.Value, logger)
			if err != nil {
				continue
			}
			familyRawBuilder.WriteString(metricRawBuilder.String()) // TODO: may need to take this out
		}
	}

	return familyRawBuilder.String()
}

// inheritMetricAttributes applies family-level labels and resolver to the metric.
func inheritMetricAttributes(f *FamilyType, metric *MetricType) {
	metric.AddonStubs = append(metric.AddonStubs, f.AddonStubs...)
}

// writeMetricSamples writes single or expanded metric values based on label structure.
func writeMetricSamples(builder *strings.Builder, name string, u *unstructured.Unstructured, keys, values []string, value float64, logger klog.Logger) error {
	writeMetric := func(k, v []string) error {
		builder.WriteString(kubeCustomResourcePrefix + name)

		return writeMetricTo(
			builder,
			u.GroupVersionKind().Group,
			u.GroupVersionKind().Version,
			u.GroupVersionKind().Kind,
			value,
			k, v,
		)
	}
	return writeSample(writeMetric, keys, values, logger)
}

// writeSample writes a single metric sample.
func writeSample(writeFunc func([]string, []string) error, keys, values []string, logger klog.Logger) error {
	if err := writeFunc(keys, values); err != nil {
		logger.V(1).Error(fmt.Errorf("error writing metric: %w", err), "skipping")

		return err
	}

	return nil
}

// buildHeaders generates the header for the given family.
func (f *FamilyType) buildHeaders() string {
	header := strings.Builder{}
	header.WriteString("# HELP " + kubeCustomResourcePrefix + f.Name + " " + f.Help)
	header.WriteString("\n")
	header.WriteString("# TYPE " + kubeCustomResourcePrefix + f.Name + " " + metricTypeGauge)

	return header.String()
}
