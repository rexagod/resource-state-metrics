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
) *StoreType {
	logger := klog.FromContext(ctx)

	// Create the reflector's LW.
	gvr := gvkWithR.GroupVersionResource
	lwo := metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	}
	resourceVersionLatestBestEffort := "0"
	if tryNoCache {
		lwo.ResourceVersionMatch = metav1.ResourceVersionMatchNotOlderThan
		lwo.ResourceVersion = resourceVersionLatestBestEffort
	}
	lw := &cache.ListWatch{
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

	// Create the reflector's StoreType.
	headers := make([]string, len(metricFamilies))
	for i, f := range metricFamilies {
		headers[i] = f.buildHeaders()
	}
	s := newStore(logger, headers, metricFamilies)

	// Create and start the reflector.
	wrapper := &unstructured.Unstructured{}
	wrapper.SetGroupVersionKind(gvkWithR.GroupVersionKind)
	reflector := cache.NewReflectorWithOptions(lw, wrapper, s, cache.ReflectorOptions{
		Name:         fmt.Sprintf("%#q reflector", gvr.String()),
		ResyncPeriod: 0,
	})
	go reflector.Run(ctx.Done())

	return s
}
