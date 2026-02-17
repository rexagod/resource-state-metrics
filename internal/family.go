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
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rexagod/resource-state-metrics/pkg/resolver"
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

// stringBuilderPool pools strings.Builder instances to reduce GC pressure
// during metric generation. It does so by cutting down on the number of
// allocations and deallocations of strings.Builder objects, which can be
// significant when generating a large number of metrics, especially in
// high-cardinality scenarios.
var stringBuilderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

func getBuilder() *strings.Builder {
	b, ok := stringBuilderPool.Get().(*strings.Builder)
	if !ok {
		return &strings.Builder{}
	}

	return b
}

func putBuilder(b *strings.Builder) {
	b.Reset()
	stringBuilderPool.Put(b)
}

// ResolverType represents the type of resolver to use to evaluate the labelset expressions.
type ResolverType string

const (
	// ResolverTypeCEL represents the CEL resolver, which uses Common Expression Language (CEL) to evaluate labelset expressions.
	ResolverTypeCEL ResolverType = "cel"
	// ResolverTypeUnstructured represents the unstructured resolver, which uses simple dot notation to resolve labelset expressions.
	ResolverTypeUnstructured ResolverType = "unstructured"
	// ResolverTypeNone represents the absence of a resolver.
	ResolverTypeNone ResolverType = ""
)

// FamilyType represents a metric family (a group of metrics with the same name).
type FamilyType struct {
	logger              klog.Logger
	celCostLimit        uint64
	celTimeout          time.Duration
	celEvaluations      *prometheus.CounterVec
	managedRMMNamespace string
	managedRMMName      string
	Name                string        `yaml:"name"`
	Help                string        `yaml:"help"`
	Metrics             []*MetricType `yaml:"metrics"`
	Resolver            ResolverType  `yaml:"resolver,omitempty"`
	LabelKeys           []string      `yaml:"labelKeys,omitempty"`
	LabelValues         []string      `yaml:"labelValues,omitempty"`
}

// buildMetricString returns the given family in its byte representation.
func (f *FamilyType) buildMetricString(unstructured *unstructured.Unstructured) string {
	logger := f.logger.WithValues("family", f.Name)
	familyRawBuilder := getBuilder()
	defer putBuilder(familyRawBuilder)

	for _, metric := range f.Metrics {
		metricRawBuilder := getBuilder()

		inheritMetricAttributes(f, metric)
		resolverInstance, err := f.resolver(metric.Resolver)
		if err != nil {
			logger.V(1).Error(fmt.Errorf("error resolving metric: %w", err), "skipping")
			putBuilder(metricRawBuilder)

			continue
		}

		resolvedLabelKeys, resolvedLabelValues, resolvedExpandedLabelSet := resolveLabels(metric, resolverInstance, unstructured.Object)

		resolvedValue, found := resolverInstance.Resolve(metric.Value, unstructured.Object)[metric.Value]
		if !found {
			logger.V(1).Error(fmt.Errorf("error resolving metric value %q", metric.Value), "skipping")
			putBuilder(metricRawBuilder)

			continue
		}

		err = writeMetricSamples(metricRawBuilder, f.Name, unstructured, resolvedLabelKeys, resolvedLabelValues, resolvedExpandedLabelSet, resolvedValue, logger)
		if err != nil {
			putBuilder(metricRawBuilder)

			continue
		}
		familyRawBuilder.WriteString(metricRawBuilder.String())
		putBuilder(metricRawBuilder)
	}

	return familyRawBuilder.String()
}

// inheritMetricAttributes applies family-level labels and resolver to the metric.
func inheritMetricAttributes(f *FamilyType, metric *MetricType) {
	metric.LabelKeys = append(metric.LabelKeys, f.LabelKeys...)
	metric.LabelValues = append(metric.LabelValues, f.LabelValues...)
}

// resolveLabels resolves label keys and values including handling of composite map/list structures.
func resolveLabels(metric *MetricType, resolverInstance resolver.Resolver, obj map[string]interface{}) ([]string, []string, map[string][]string) {
	var (
		resolvedLabelKeys        []string
		resolvedLabelValues      []string
		resolvedExpandedLabelSet = make(map[string][]string)
	)

	// Validate that label keys and values have the same length before indexing.
	if err := validateLabelLengths(metric.LabelKeys, metric.LabelValues); err != nil {
		klog.Error(err, "skipping metric due to label length mismatch")
		// Return empty results on validation failure to skip this metric gracefully.
		return resolvedLabelKeys, resolvedLabelValues, resolvedExpandedLabelSet
	}

	for queryIndex, query := range metric.LabelValues {
		resolvedLabelset := resolverInstance.Resolve(query, obj)
		// If the query is found in the resolved labelset, it means we are dealing with non-composite value(s).
		// For e.g., consider:
		// * `name: o.metadata.name` -> `o.metadata.name: foo`
		// * `v: o.spec.versions` -> `v#0: [v1, v2]` // no `o.spec.versions` in the resolved labelset
		if val, ok := resolvedLabelset[query]; ok {
			resolvedLabelValues = append(resolvedLabelValues, val)
			resolvedLabelKeys = append(resolvedLabelKeys, sanitizeKey(metric.LabelKeys[queryIndex]))
		} else {
			for k, v := range resolvedLabelset {
				// Check if key has a suffix that satisfies the regex: "#\d+".
				// This is used to identify list values in way that's resolver-agnostic.
				if regexp.MustCompile(`.+#\d+`).MatchString(k) {
					key := k[:strings.LastIndex(k, "#")]
					// If `o.spec.tags` is a list, the labelset will look like `metric_name{tags="tagX"}`,
					// where the number of generated samples will be same as the length of the list.
					resolvedExpandedLabelSet[key] = append(resolvedExpandedLabelSet[key], v)

					continue
				}
				resolvedLabelValues = append(resolvedLabelValues, v)
				resolvedLabelKeys = append(resolvedLabelKeys, sanitizeKey(metric.LabelKeys[queryIndex]+k))
			}
		}
	}

	return resolvedLabelKeys, resolvedLabelValues, resolvedExpandedLabelSet
}

// sanitizeKey converts a label key to snake_case and strips non-alphanumeric characters.
func sanitizeKey(s string) string {
	return strcase.ToSnake(regexp.MustCompile(`\W`).ReplaceAllString(s, "_"))
}

// writeMetricSamples writes single or expanded metric values based on label structure.
func writeMetricSamples(builder *strings.Builder, name string, u *unstructured.Unstructured, keys, values []string, expanded map[string][]string, value string, logger klog.Logger) error {
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
	if len(expanded) == 0 {
		return writeSingleSample(writeMetric, keys, values, logger)
	}

	return writeExpandedSamples(writeMetric, keys, values, expanded, logger)
}

// writeSingleSample writes a single metric sample.
func writeSingleSample(writeFunc func([]string, []string) error, keys, values []string, logger klog.Logger) error {
	if err := writeFunc(keys, values); err != nil {
		logger.V(1).Error(fmt.Errorf("error writing metric: %w", err), "skipping")

		return err
	}

	return nil
}

// writeExpandedSamples writes metric samples for list-based label values.
func writeExpandedSamples(writeFunc func([]string, []string) error, labelKeys, labelValues []string, expanded map[string][]string, logger klog.Logger) error {
	var seriesToGenerate int

	for k := range expanded {
		labelKeys = append(labelKeys, k)
		if len(expanded[k]) > seriesToGenerate {
			seriesToGenerate = len(expanded[k])
		}
		slices.Sort(expanded[k])
	}

	for range seriesToGenerate {
		ephemeralLabelValues := labelValues
		// Don't iterate over the `expanded` map, as the order of keys is unstable.
		expansionKeys := labelKeys[len(labelKeys)-len(expanded):]
		for _, k := range expansionKeys {
			vs := expanded[k]
			if len(vs) == 0 {
				ephemeralLabelValues = append(ephemeralLabelValues, "")

				continue
			}
			ephemeralLabelValues = append(ephemeralLabelValues, vs[0])
			expanded[k] = vs[1:]
		}
		// Pass a copy of the label keys and values to avoid modifying the original slices.
		if err := writeFunc(slices.Clone(labelKeys), slices.Clone(ephemeralLabelValues)); err != nil {
			logger.V(1).Error(fmt.Errorf("error writing metric: %w", err), "skipping")

			return err
		}
	}

	return nil
}

func (f *FamilyType) resolver(inheritedResolver ResolverType) (resolver.Resolver, error) {
	if inheritedResolver == ResolverTypeNone {
		inheritedResolver = f.Resolver
	}
	switch inheritedResolver {
	case ResolverTypeNone:
		fallthrough // Default to Unstructured resolver.
	case ResolverTypeUnstructured:
		return resolver.NewUnstructuredResolver(f.logger), nil
	case ResolverTypeCEL:
		return resolver.NewCELResolver(f.logger, f.celCostLimit, f.celTimeout, f.celEvaluations, f.managedRMMNamespace, f.managedRMMName, f.Name), nil
	default:
		return nil, fmt.Errorf("error resolving metric: unknown resolver %q", inheritedResolver)
	}
}

// buildHeaders generates the header for the given family.
func (f *FamilyType) buildHeaders() string {
	header := strings.Builder{}
	header.WriteString("# HELP " + kubeCustomResourcePrefix + f.Name + " " + f.Help)
	header.WriteString("\n")
	header.WriteString("# TYPE " + kubeCustomResourcePrefix + f.Name + " " + metricTypeGauge)

	return header.String()
}
