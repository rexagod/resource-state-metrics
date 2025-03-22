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
	tryNoCache bool,
	labelSelector, fieldSelector string,
	resolver ResolverType,
	labelKeys []string, labelValues []string,
) *StoreType {
	logger := klog.FromContext(ctx)

	// Create the reflector's LW.
	listerwatcher := buildLW(ctx, dynamicClientset, labelSelector, fieldSelector, tryNoCache, gvkWithR.GroupVersionResource)

	// Build metric headers.
	headers := make([]string, len(metricFamilies))
	for i, f := range metricFamilies {
		headers[i] = f.buildHeaders()
	}

	// Set the default resolver.
	if resolver == ResolverTypeNone {
		resolver = ResolverTypeUnstructured
	}

	// Instantiate a new store.
	s := newStore(
		logger,
		headers,
		metricFamilies,
		resolver,
		labelKeys, labelValues,
	)

	// Create and start the reflector.
	wrapper := &unstructured.Unstructured{}
	wrapper.SetGroupVersionKind(gvkWithR.GroupVersionKind)
	reflector := cache.NewReflectorWithOptions(listerwatcher, wrapper, s, cache.ReflectorOptions{
		Name: fmt.Sprintf("%#q reflector", gvkWithR.GroupVersionResource.String()),
	})
	go reflector.Run(ctx.Done())

	return s
}

func buildLW(
	ctx context.Context,
	dynamicClientset dynamic.Interface,
	labelSelector string,
	fieldSelector string,
	tryNoCache bool,
	gvr schema.GroupVersionResource,
) *cache.ListWatch {
	lwo := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}
	if tryNoCache {
		resourceVersionLatestBestEffort := "0"
		lwo.ResourceVersion = resourceVersionLatestBestEffort
		lwo.ResourceVersionMatch = metav1.ResourceVersionMatchNotOlderThan
	}
	listerwatcher := &cache.ListWatch{
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

	return listerwatcher
}
