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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// gvkr holds the GVK/R information for the custom resource that the store is built for.
type gvkr struct {
	schema.GroupVersionKind
	schema.GroupVersionResource
}

// buildStore builds a cache.store for the metrics store.
func buildStore(
	ctx context.Context,
	dynamicClientset dynamic.Interface,
	gvkWithR gvkr,
	metricFamilies []*FamilyType,
	labelSelector, fieldSelector string,
	resolver ResolverType,
	labelKeys, labelValues []string,
	celCostLimit uint64,
	celTimeout time.Duration,
) *StoreType {
	logger := klog.FromContext(ctx)
	listerwatcher := buildLW(ctx, dynamicClientset, labelSelector, fieldSelector, gvkWithR.GroupVersionResource)
	headers := buildMetricHeaders(metricFamilies)
	resolver = ensureResolver(resolver)
	// Propagate CEL limits to all families
	for _, family := range metricFamilies {
		family.celCostLimit = celCostLimit
		family.celTimeout = celTimeout
	}
	s := newStore(logger, headers, metricFamilies, resolver, labelKeys, labelValues, celCostLimit, celTimeout)
	startReflector(ctx, listerwatcher, gvkWithR, s)

	return s
}

func buildMetricHeaders(metricFamilies []*FamilyType) []string {
	headers := make([]string, len(metricFamilies))
	for i, f := range metricFamilies {
		headers[i] = f.buildHeaders()
	}

	return headers
}

func ensureResolver(resolver ResolverType) ResolverType {
	if resolver == ResolverTypeNone {
		return ResolverTypeUnstructured
	}

	return resolver
}

func startReflector(ctx context.Context, lw *cache.ListWatch, gvkWithR gvkr, s *StoreType) {
	wrapper := &unstructured.Unstructured{}
	wrapper.SetGroupVersionKind(gvkWithR.GroupVersionKind)

	reflector := cache.NewReflectorWithOptions(lw, wrapper, s, cache.ReflectorOptions{
		Name: fmt.Sprintf("%#q reflector", gvkWithR.GroupVersionResource.String()),
	})

	go reflector.Run(ctx.Done())
}

func buildLW(
	ctx context.Context,
	dynamicClientset dynamic.Interface,
	labelSelector string,
	fieldSelector string,
	gvr schema.GroupVersionResource,
) *cache.ListWatch {
	lwo := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}

	return &cache.ListWatch{
		ListFunc: func(_ metav1.ListOptions) (runtime.Object, error) {
			o, err := dynamicClientset.Resource(gvr).List(ctx, lwo)
			if err != nil {
				err = fmt.Errorf("error listing %s with options %v: %w", gvr.String(), lwo, err)
			}

			return o, err
		},
		WatchFunc: func(_ metav1.ListOptions) (watch.Interface, error) {
			o, err := dynamicClientset.Resource(gvr).Watch(ctx, lwo)
			if err != nil {
				err = fmt.Errorf("error watching %s with options %v: %w", gvr.String(), lwo, err)
			}

			return o, err
		},
	}
}
