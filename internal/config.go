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

	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// configure defines behaviours for working with configuration(s).
type configure interface {
	// parse parses the given configuration.
	parse(raw string) error

	// build builds the given configuration.
	build(ctx context.Context, uidToStoresMap map[types.UID][]*StoreType, tryNoCache bool)
}

// configuration defines the structured representation of a YAML configuration.
type configuration struct {
	Stores []*StoreType `yaml:"stores"`
}

// configurer knows how to parse a YAML configuration.
type configurer struct {
	configuration    configuration
	dynamicClientset dynamic.Interface
	resource         *v1alpha1.ResourceMetricsMonitor
}

// Ensure configurer implements configure.
var _ configure = &configurer{}

// newConfigurer returns a new configurer.
func newConfigurer(dynamicClientset dynamic.Interface, resource *v1alpha1.ResourceMetricsMonitor) *configurer {
	return &configurer{
		dynamicClientset: dynamicClientset,
		resource:         resource,
	}
}

// parse unmarshals the raw YAML configuration.
func (c *configurer) parse(raw string) error {
	if err := yaml.Unmarshal([]byte(raw), &c.configuration); err != nil {
		return fmt.Errorf("error unmarshalling configuration: %w", err)
	}

	return nil
}

// build constructs the metric stores from the parsed configuration.
func (c *configurer) build(ctx context.Context, uidToStoresMap map[types.UID][]*StoreType, tryNoCache bool) {
	for _, cfg := range c.configuration.Stores {
		s := c.buildStoreFromConfig(ctx, cfg, tryNoCache)
		resourceUID := c.resource.GetUID()
		uidToStoresMap[resourceUID] = append(uidToStoresMap[resourceUID], s)
	}
}

func (c *configurer) buildStoreFromConfig(ctx context.Context, cfg *StoreType, tryNoCache bool) *StoreType {
	gvkWithR := buildGVKR(cfg)

	return buildStore(
		ctx,
		c.dynamicClientset,
		gvkWithR,
		cfg.Families,
		tryNoCache,
		cfg.Selectors.Label, cfg.Selectors.Field,
		cfg.Resolver,
		cfg.LabelKeys, cfg.LabelValues,
	)
}

func buildGVKR(cfg *StoreType) gvkr {
	return gvkr{
		GroupVersionKind: schema.GroupVersionKind{
			Group:   cfg.Group,
			Version: cfg.Version,
			Kind:    cfg.Kind,
		},
		GroupVersionResource: schema.GroupVersionResource{
			Group:    cfg.Group,
			Version:  cfg.Version,
			Resource: cfg.ResourceName,
		},
	}
}
