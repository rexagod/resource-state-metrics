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
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
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

// ResolverType represents the type of resolver to use to evaluate the labelset expressions.
type ResolverType string

const (

	// ResolverTypeCEL represents the CEL resolver.
	ResolverTypeCEL ResolverType = "cel"

	// ResolverTypeUnstructured represents the Unstructured resolver.
	ResolverTypeUnstructured ResolverType = "unstructured"

	// ResolverTypeNone represents an empty resolver.
	ResolverTypeNone ResolverType = ""
)

// FamilyType represents a metric family (a group of metrics with the same name).
type FamilyType struct {

	// logger is the family's logger.
	logger klog.Logger

	// Name is the Name of the metric family.
	Name string `yaml:"name"`

	// Help is the Help text for the metric family.
	Help string `yaml:"help"`

	// t is the type of the metric family.
	// NOTE: This will always be pinned to `gauge`, and thus not exported for unmarshalling.
	t string

	// Metrics is a slice of Metrics that belong to the MetricType family.
	Metrics []*MetricType `yaml:"metrics"`

	// Resolver is the resolver to use to evaluate the labelset expressions.
	Resolver ResolverType `yaml:"resolver"`

	// LabelKeys is the set of inherited or defined label keys.
	LabelKeys []string `yaml:"labelKeys,omitempty"`

	// LabelValues is the set of inherited or defined label values.
	LabelValues []string `yaml:"labelValues,omitempty"`
}

// buildMetricString returns the given family in its byte representation.
func (f *FamilyType) buildMetricString(unstructured *unstructured.Unstructured) string {
	logger := f.logger.WithValues("family", f.Name)

	familyRawBuilder := strings.Builder{}
	for _, metric := range f.Metrics {
		metricRawBuilder := strings.Builder{}

		// Inherit the label keys and values.
		metric.LabelKeys = append(metric.LabelKeys, f.LabelKeys...)
		metric.LabelValues = append(metric.LabelValues, f.LabelValues...)

		// Inherit the resolver.
		resolverInstance, err := f.resolver(metric.Resolver)
		if err != nil {
			logger.V(1).Error(fmt.Errorf("error resolving metric: %w", err), "skipping")

			continue
		}

		// Resolve the labelset.
		var (
			resolvedLabelKeys   []string
			resolvedLabelValues []string
		)
		for i, query := range metric.LabelValues {
			resolvedLabelset := resolverInstance.Resolve(query, unstructured.Object)

			// If the query is found in the resolved labelset, append the resolved value.
			if resolvedLabelValue, ok := resolvedLabelset[query]; ok {
				resolvedLabelValues = append(resolvedLabelValues, resolvedLabelValue)

				// Label keys are not resolved if the returned labelset for the same label key exists.
				resolvedLabelKeys = append(resolvedLabelKeys, strings.ToLower(regexp.MustCompile(`\W`).
					ReplaceAllString(metric.LabelKeys[i], "_")))

				// If the query is not found in the resolved labelset, it is now redundant as a label value.
			} else {
				for k, v := range resolvedLabelset {
					resolvedLabelValues = append(resolvedLabelValues, v)

					// Label keys are resolved (with the original label keys being the new label key's prefix) if the
					// returned labelset for the same label key does not exist.
					resolvedLabelKeys = append(resolvedLabelKeys, strcase.ToLowerCamel(regexp.MustCompile(`\W`).
						ReplaceAllString(metric.LabelKeys[i]+k, "_")))
				}
			}
		}

		// Resolve the metric value.
		resolvedValue, found := resolverInstance.Resolve(metric.Value, unstructured.Object)[metric.Value]
		if !found {
			logger.V(1).Error(fmt.Errorf("error resolving metric value %q", metric.Value), "skipping")

			continue
		}

		// Write the metric.
		metricRawBuilder.WriteString(kubeCustomResourcePrefix)
		metricRawBuilder.WriteString(f.Name)
		err = writeMetricTo(
			&metricRawBuilder,
			unstructured.GroupVersionKind().Group, unstructured.GroupVersionKind().Version, unstructured.GroupVersionKind().Kind,
			resolvedValue,
			resolvedLabelKeys, resolvedLabelValues,
		)
		if err != nil {
			logger.V(1).Error(fmt.Errorf("error writing metric: %w", err), "skipping")

			continue
		}

		familyRawBuilder.WriteString(metricRawBuilder.String())
	}

	return familyRawBuilder.String()
}

func (f *FamilyType) resolver(inheritedResolver ResolverType) (resolver.Resolver, error) {
	var resolverInstance resolver.Resolver
	if inheritedResolver == ResolverTypeNone {
		inheritedResolver = f.Resolver
	}
	switch inheritedResolver {
	case ResolverTypeNone:
		fallthrough // Default to Unstructured resolver.
	case ResolverTypeUnstructured:
		resolverInstance = resolver.NewUnstructuredResolver(f.logger)
	case ResolverTypeCEL:
		resolverInstance = resolver.NewCELResolver(f.logger)
	default:
		return nil, fmt.Errorf("error resolving metric: unknown resolver %q", inheritedResolver)
	}

	return resolverInstance, nil
}

// buildHeaders generates the header for the given family.
func (f *FamilyType) buildHeaders() string {
	header := strings.Builder{}

	// Write the help text.
	header.WriteString("# HELP ")
	header.WriteString(kubeCustomResourcePrefix)
	header.WriteString(f.Name)
	header.WriteString(" ")
	header.WriteString(f.Help)
	header.WriteString("\n")

	// Write the type text.
	header.WriteString("# TYPE ")
	header.WriteString(kubeCustomResourcePrefix)
	header.WriteString(f.Name)
	header.WriteString(" ")
	f.t = metricTypeGauge
	header.WriteString(f.t)

	return header.String()
}
