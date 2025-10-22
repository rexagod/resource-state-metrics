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
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/traefik/yaegi/stdlib"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"

	"github.com/traefik/yaegi/interp"
)

// MetricType represents a single time series.
type MetricType struct {
	AddonStubs []string `yaml:"addonStubs,omitempty"`
	Stubs      []string `yaml:"stubs,omitempty"`
}

type SampleType struct {
	LabelKeys   []string `yaml:"-"`
	LabelValues []string `yaml:"-"`
	Value       float64  `yaml:"-"`
}

func (m *MetricType) resolve(logger klog.Logger, unstructured *unstructured.Unstructured) []SampleType {
	var additionalSamples []SampleType
	for _, addonStub := range m.AddonStubs {
		stubSamples, err := executeStub(addonStub, unstructured)
		if err != nil {
			logger.Error(err, "Failed to execute addon stub, skipping", "addonStub", addonStub)
			continue
		}
		additionalSamples = append(additionalSamples, stubSamples...)
	}
	var allAdditionalSamplesMerged SampleType
	for _, additionalSample := range additionalSamples {
		allAdditionalSamplesMerged.LabelKeys = append(allAdditionalSamplesMerged.LabelKeys, additionalSample.LabelKeys...)
		allAdditionalSamplesMerged.LabelValues = append(allAdditionalSamplesMerged.LabelValues, additionalSample.LabelValues...)
	}
	var samples []SampleType
	for _, stub := range m.Stubs {
		stubSamples, err := executeStub(stub, unstructured)
		if err != nil {
			logger.Error(err, "Failed to execute stub, skipping", "stub", stub)
			continue
		}
		for i := range stubSamples {
			stubSamples[i].LabelKeys = append(stubSamples[i].LabelKeys, allAdditionalSamplesMerged.LabelKeys...)
			stubSamples[i].LabelValues = append(stubSamples[i].LabelValues, allAdditionalSamplesMerged.LabelValues...)
		}
		samples = append(samples, stubSamples...)
	}

	return samples
}

func executeStub(stub string, unstructuredTyped *unstructured.Unstructured) ([]SampleType, error) {
	timeout := 5 * time.Second
	ctx, cancelFn := context.WithTimeout(context.WithValue(context.Background(), "timeout", timeout), timeout)
	defer cancelFn()

	interpreter := interp.New(interp.Options{})
	err := interpreter.Use(stdlib.Symbols)
	if err != nil {
		panic(err)
	}
	err = interpreter.Use(interp.Exports{
		// Yaegi uses "path/packagename" format.
		"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructured": map[string]reflect.Value{
			"Unstructured": reflect.ValueOf((*unstructured.Unstructured)(nil)),
		},
		"github.com/kubernetes-sigs/resource-state-metrics/pkg/utils/utils": map[string]reflect.Value{
			"SampleType": reflect.ValueOf((*SampleType)(nil)),
		},
		"k8s.io/klog/v2/v2": map[string]reflect.Value{
			"InfoS":  reflect.ValueOf(klog.InfoS),
			"Error":  reflect.ValueOf(klog.Error),
			"ErrorS": reflect.ValueOf(klog.ErrorS),
		},
	})
	if err != nil {
		panic(err)
	}
	_, err = interpreter.EvalWithContext(ctx, stub)
	if err != nil {
		return nil, fmt.Errorf("error evaluating stub: %w", err)
	}
	samples, err := interpreter.EvalWithContext(ctx, "foo.samples")
	if err != nil {
		return nil, fmt.Errorf("error extracting samples from stub: %w", err)
	}
	if !samples.CanInterface() {
		return nil, fmt.Errorf("unable to interface stub result")
	}
	samplesInterface := samples.Interface()
	samplesFn, ok := samplesInterface.(func(*unstructured.Unstructured) []SampleType)
	if !ok {
		return nil, fmt.Errorf("expected stub result to be of type []SampleType but got %T", samplesInterface)
	}
	resolvedSamples := samplesFn(unstructuredTyped)

	return resolvedSamples, nil
}

func writeMetricTo(writer *strings.Builder, g, v, k string, resolvedValue float64, resolvedLabelKeys, resolvedLabelValues []string) error {
	if err := validateLabelLengths(resolvedLabelKeys, resolvedLabelValues); err != nil {
		return fmt.Errorf("key and label lengths do not match: %w", err)
	}
	// sortLabelset(resolvedLabelKeys, resolvedLabelValues) // Do this in tests only.
	resolvedLabelKeys, resolvedLabelValues = appendGVKLabels(resolvedLabelKeys, resolvedLabelValues, g, v, k)
	if err := writeLabels(writer, resolvedLabelKeys, resolvedLabelValues); err != nil {
		return fmt.Errorf("error writing labels: %w", err)
	}

	return writeValue(writer, resolvedValue)
}

func validateLabelLengths(keys, values []string) error {
	if len(keys) != len(values) {
		return fmt.Errorf(
			"expected labelKeys %q to be of same length (%d) as the resolved labelValues %q (%d)",
			keys, len(keys), values, len(values),
		)
	}

	return nil
}

func appendGVKLabels(keys, values []string, g, v, k string) ([]string, []string) {
	keys = append(keys, "group", "version", "kind")
	values = append(values, g, v, k)

	return keys, values
}

func writeLabels(writer *strings.Builder, keys, values []string) error {
	if len(keys) == 0 {
		return nil
	}

	separator := "{"
	for i := range keys {
		writer.WriteString(separator)
		writer.WriteString(keys[i])
		writer.WriteString("=\"")
		n, err := strings.NewReplacer("\\", `\\`, "\n", `\n`, "\"", `\"`).WriteString(writer, values[i])
		if err != nil {
			return fmt.Errorf("error writing metric after %d bytes: %w", n, err)
		}
		writer.WriteString("\"")
		separator = ","
	}
	writer.WriteString("}")

	return nil
}

func writeValue(writer *strings.Builder, value float64) error {
	writer.WriteByte(' ')
	n, err := fmt.Fprintf(writer, "%f", value)
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
