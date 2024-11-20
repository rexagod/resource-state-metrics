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
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// StoreType implements the k8s.io/client-go/tools/cache.StoreType interface. The cache.Reflector uses the cache.StoreType to
// operate on the store.metrics map with the various metric families and their metrics based on the associated object's
// events.
type StoreType struct {

	// logger is the store's logger.
	logger klog.Logger

	// mutex is a binary semaphore that is used to prevent RW races w.r.t. the store's internal metric map.
	mutex sync.RWMutex

	// metrics is the store's internal metric map. It is indexed by the object's UID and contains a slice of
	// metric families, which in turn contain a slice of metrics.
	metrics map[types.UID][]string

	// headers contain the type and help text for each metric family, corresponding to the store's internal
	// metric map's keys.
	headers []string

	// ==================================================================================================
	// Exported attributes that each store is associated with, used for unmarshalling the configuration.
	// ==================================================================================================

	// Group is the API group of the custom resource.
	Group string `yaml:"g"`

	// Version is the API version of the custom resource.
	Version string `yaml:"v"`

	// Kind is the type of the custom resource.
	Kind string `yaml:"k"`

	// ResourceName is the name (plural) of the custom resource, in lowercase.
	ResourceName string `yaml:"r"`

	// Selectors is the selectors to use to filter the objects.
	Selectors struct {
		Label string `yaml:"label,omitempty"`
		Field string `yaml:"field,omitempty"`
	} `yaml:"selectors,omitempty"`

	// Families is a slice of metric families.
	Families []*FamilyType `yaml:"families"`

	// Resolver is the resolver to use to evaluate expressions.
	Resolver ResolverType `yaml:"resolver"`

	// LabelKeys is a slice of label keys.
	LabelKeys []string `yaml:"labelKeys,omitempty"`

	// LabelValues is a slice of label values.
	LabelValues []string `yaml:"labelValues,omitempty"`
}

// newStore returns a new store.
func newStore(
	logger klog.Logger,
	headers []string,
	families []*FamilyType,
	resolver ResolverType,
	labelKeys []string, labelValues []string,
) *StoreType {
	return &StoreType{
		logger:      logger,
		metrics:     map[types.UID][]string{},
		headers:     headers,
		Families:    families,
		Resolver:    resolver,
		LabelKeys:   labelKeys,
		LabelValues: labelValues,
	}
}

// Add adds the given object to the accumulator associated with its key.
func (s *StoreType) Add(objectI interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Convert into an unstructured object.
	unstructuredObjectMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(objectI)
	if err != nil {
		return fmt.Errorf("error converting object interface to unstructured: %w", err)
	}
	unstructuredObject := &unstructured.Unstructured{Object: unstructuredObjectMap}

	// Generate metrics from the object.
	familyMetrics := make([]string, len(s.Families))
	for i, f := range s.Families {
		// Inherit the resolver.
		if f.Resolver == ResolverTypeNone {
			f.Resolver = s.Resolver
		}

		// Inherit the label keys and values.
		f.LabelKeys = append(f.LabelKeys, s.LabelKeys...)
		f.LabelValues = append(f.LabelValues, s.LabelValues...)

		// Generate the metrics.
		f.logger = s.logger
		familyMetrics[i] = f.rawFrom(unstructuredObject)
		s.logger.V(4).Info("Add", "family", f.Name, "metrics", familyMetrics[i])
	}

	// Store the generated metrics.
	s.logger.V(2).Info("Add", "key", klog.KObj(unstructuredObject))
	s.metrics[unstructuredObject.GetUID()] = familyMetrics

	return nil
}

// Update updates the given object in the accumulator associated with its key.
func (s *StoreType) Update(objectI interface{}) error {
	s.logger.V(2).Info("Update", "defer", "Add")

	return s.Add(objectI)
}

// Delete deletes the given object from the accumulator associated with its key.
func (s *StoreType) Delete(objectI interface{}) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cast into a typed object.
	object, err := meta.Accessor(objectI)
	if err != nil {
		return fmt.Errorf("error casting object interface: %w", err)
	}

	// Delete the object's metrics.
	s.logger.V(2).Info("Delete", "key", klog.KObj(object))
	s.logger.V(4).Info("Delete", "metrics", s.metrics[object.GetUID()])
	delete(s.metrics, object.GetUID())

	return nil
}

// List returns a list of all the currently non-empty accumulators.
func (s *StoreType) List() []interface{} {
	return nil
}

// ListKeys returns a list of all the keys of the currently non-empty accumulators.
func (s *StoreType) ListKeys() []string {
	return nil
}

// Get returns the accumulator associated with the given object's key.
func (s *StoreType) Get(_ interface{}) (interface{}, bool, error) {
	return nil, false, nil
}

// GetByKey returns the accumulator associated with the given key.
func (s *StoreType) GetByKey(_ string) (interface{}, bool, error) {
	return nil, false, nil
}

// Replace will delete the contents of the store, using instead the given list. store takes ownership of the list, you
// should not reference it after calling this function.
// NOTE: cache.Reflector starts off with Replace followed by Add rather than just Add, and as such this is skipped to
// avoid building stores twice.
func (s *StoreType) Replace(_ []interface{}, _ string) error {
	return nil
}

// Resync is meaningless in the terms appearing here but has meaning in some implementations that have non-trivial
// additional behavior (e.g., DeltaFIFO).
func (s *StoreType) Resync() error {
	return nil
}
