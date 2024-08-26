/*
Copyright 2024 The Kubernetes crdmetrics Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	crdmetricsv1alpha1 "github.com/rexagod/crdmetrics/pkg/apis/crdmetrics/v1alpha1"
	versioned "github.com/rexagod/crdmetrics/pkg/generated/clientset/versioned"
	internalinterfaces "github.com/rexagod/crdmetrics/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/rexagod/crdmetrics/pkg/generated/listers/crdmetrics/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CRDMetricsResourceInformer provides access to a shared informer and lister for
// CRDMetricsResources.
type CRDMetricsResourceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CRDMetricsResourceLister
}

type cRDMetricsResourceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCRDMetricsResourceInformer constructs a new informer for CRDMetricsResource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCRDMetricsResourceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCRDMetricsResourceInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCRDMetricsResourceInformer constructs a new informer for CRDMetricsResource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCRDMetricsResourceInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CrdmetricsV1alpha1().CRDMetricsResources(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CrdmetricsV1alpha1().CRDMetricsResources(namespace).Watch(context.TODO(), options)
			},
		},
		&crdmetricsv1alpha1.CRDMetricsResource{},
		resyncPeriod,
		indexers,
	)
}

func (f *cRDMetricsResourceInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCRDMetricsResourceInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *cRDMetricsResourceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&crdmetricsv1alpha1.CRDMetricsResource{}, f.defaultInformer)
}

func (f *cRDMetricsResourceInformer) Lister() v1alpha1.CRDMetricsResourceLister {
	return v1alpha1.NewCRDMetricsResourceLister(f.Informer().GetIndexer())
}
