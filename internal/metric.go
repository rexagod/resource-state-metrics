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
	"sort"
	"strconv"
	"strings"
)

// MetricType represents a single time series.
type MetricType struct {

	// LabelKeys is the set of label keys.
	LabelKeys []string `yaml:"labelKeys"`

	// LabelValues is the set of label values.
	LabelValues []string `yaml:"labelValues"`

	// Value is the metric Value.
	Value string `yaml:"value"`

	// Resolver is the resolver to use to evaluate the labelset expressions.
	Resolver ResolverType `yaml:"resolver"`
}

// writeMetricTo writes the given metric to the given strings.Builder.
func writeMetricTo(writer *strings.Builder, g, v, k, resolvedValue string, resolvedLabelKeys, resolvedLabelValues []string) error {
	if len(resolvedLabelKeys) != len(resolvedLabelValues) {
		return fmt.Errorf(
			"expected labelKeys %q to be of same length (%d) as the resolved labelValues %q (%d)",
			resolvedLabelKeys, len(resolvedLabelKeys), resolvedLabelValues, len(resolvedLabelValues),
		)
	}

	// Sort the label keys and values. This preserves order and helps test deterministically.
	sortLabelset(resolvedLabelKeys, resolvedLabelValues)

	// Append GVK metadata to the metric.
	resolvedLabelKeys = append(resolvedLabelKeys, "group", "version", "kind")
	resolvedLabelValues = append(resolvedLabelValues, g, v, k)

	// Write the metric.
	if len(resolvedLabelKeys) > 0 {
		separator := "{"
		for i := range resolvedLabelKeys {
			writer.WriteString(separator)
			writer.WriteString(resolvedLabelKeys[i])
			writer.WriteString("=\"")
			n, err := strings.NewReplacer("\\", `\\`, "\n", `\n`, "\"", `\"`).WriteString(writer, resolvedLabelValues[i])
			if err != nil {
				return fmt.Errorf("error writing metric after %d bytes: %w", n, err)
			}
			writer.WriteString("\"")
			separator = ","
		}
		writer.WriteString("}")
	}
	writer.WriteByte(' ')
	metricValueAsFloat, err := strconv.ParseFloat(resolvedValue, 64)
	if err != nil {
		return fmt.Errorf("error parsing metric value %q as float64: %w", resolvedValue, err)
	}
	n, err := fmt.Fprintf(writer, "%f", metricValueAsFloat)
	if err != nil {
		return fmt.Errorf("error writing (float64) metric value after %d bytes: %w", n, err)
	}
	writer.WriteByte('\n')

	return nil
}

// sortLabelset sorts the label keys and values while preserving order.
func sortLabelset(resolvedLabelKeys, resolvedLabelValues []string) {
	// Populate.
	type labelset struct {
		labelKey   string
		labelValue string
	}
	labelsets := make([]labelset, len(resolvedLabelKeys))
	for i := range resolvedLabelKeys {
		labelsets[i] = labelset{labelKey: resolvedLabelKeys[i], labelValue: resolvedLabelValues[i]}
	}

	// Sort.
	sort.Slice(labelsets, func(i, j int) bool {
		a, b := labelsets[i].labelKey, labelsets[j].labelKey
		if len(a) == len(b) {
			return a < b
		}

		return len(a) < len(b)
	})

	// Re-populate.
	for i := range labelsets {
		resolvedLabelKeys[i] = labelsets[i].labelKey
		resolvedLabelValues[i] = labelsets[i].labelValue
	}
}
