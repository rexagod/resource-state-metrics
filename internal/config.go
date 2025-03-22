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

	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// configure defines behaviours for working with configuration(s), can be implemented to use configurations other than
// the CEL one.
type configure interface {

	// Parse parses the given configuration.
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

	// configuration is the structured configuration.
	configuration configuration

	// dynamicClientset is the dynamic clientset used to build stores for different objects.
	dynamicClientset dynamic.Interface

	// resource is the resource to build stores for.
	resource *v1alpha1.ResourceMetricsMonitor
}

// configurer implements the configure interface.
var _ configure = &configurer{}

// newConfigurer returns a new configurer.
func newConfigurer(
	dynamicClientset dynamic.Interface,
	resource *v1alpha1.ResourceMetricsMonitor,
) *configurer {
	return &configurer{
		dynamicClientset: dynamicClientset,
		resource:         resource,
	}
}

// parse knows how to parse the given configuration.
func (c *configurer) parse(raw string) error {
	err := yaml.Unmarshal([]byte(raw), &c.configuration)
	if err != nil {
		err = fmt.Errorf("error unmarshalling configuration: %w", err)
	}

	return err
}

// build knows how to build the given configuration.
func (c *configurer) build(ctx context.Context, uidToStoresMap map[types.UID][]*StoreType, tryNoCache bool) {
	for _, storeConfiguration := range c.configuration.Stores {
		g, v, k, r := storeConfiguration.Group, storeConfiguration.Version, storeConfiguration.Kind, storeConfiguration.ResourceName
		gvkWithR := gvkr{
			GroupVersionKind:     schema.GroupVersionKind{Group: g, Version: v, Kind: k},
			GroupVersionResource: schema.GroupVersionResource{Group: g, Version: v, Resource: r},
		}
		ls, fs := storeConfiguration.Selectors.Label, storeConfiguration.Selectors.Field
		families := storeConfiguration.Families
		resolver := storeConfiguration.Resolver
		labelKeys, labelValues := storeConfiguration.LabelKeys, storeConfiguration.LabelValues
		s := buildStore(
			ctx, c.dynamicClientset,
			gvkWithR,
			families,
			tryNoCache,
			ls, fs,
			resolver,
			labelKeys, labelValues,
		)
		resourceUID := c.resource.GetUID()
		uidToStoresMap[resourceUID] = append(uidToStoresMap[resourceUID], s)
	}
}
