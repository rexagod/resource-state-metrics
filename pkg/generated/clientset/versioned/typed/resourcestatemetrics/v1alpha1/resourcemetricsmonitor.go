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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	context "context"

	resourcestatemetricsv1alpha1 "github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	scheme "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// ResourceMetricsMonitorsGetter has a method to return a ResourceMetricsMonitorInterface.
// A group's client should implement this interface.
type ResourceMetricsMonitorsGetter interface {
	ResourceMetricsMonitors(namespace string) ResourceMetricsMonitorInterface
}

// ResourceMetricsMonitorInterface has methods to work with ResourceMetricsMonitor resources.
type ResourceMetricsMonitorInterface interface {
	Create(ctx context.Context, resourceMetricsMonitor *resourcestatemetricsv1alpha1.ResourceMetricsMonitor, opts v1.CreateOptions) (*resourcestatemetricsv1alpha1.ResourceMetricsMonitor, error)
	Update(ctx context.Context, resourceMetricsMonitor *resourcestatemetricsv1alpha1.ResourceMetricsMonitor, opts v1.UpdateOptions) (*resourcestatemetricsv1alpha1.ResourceMetricsMonitor, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, resourceMetricsMonitor *resourcestatemetricsv1alpha1.ResourceMetricsMonitor, opts v1.UpdateOptions) (*resourcestatemetricsv1alpha1.ResourceMetricsMonitor, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*resourcestatemetricsv1alpha1.ResourceMetricsMonitor, error)
	List(ctx context.Context, opts v1.ListOptions) (*resourcestatemetricsv1alpha1.ResourceMetricsMonitorList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *resourcestatemetricsv1alpha1.ResourceMetricsMonitor, err error)
	ResourceMetricsMonitorExpansion
}

// resourceMetricsMonitors implements ResourceMetricsMonitorInterface
type resourceMetricsMonitors struct {
	*gentype.ClientWithList[*resourcestatemetricsv1alpha1.ResourceMetricsMonitor, *resourcestatemetricsv1alpha1.ResourceMetricsMonitorList]
}

// newResourceMetricsMonitors returns a ResourceMetricsMonitors
func newResourceMetricsMonitors(c *ResourceStateMetricsV1alpha1Client, namespace string) *resourceMetricsMonitors {
	return &resourceMetricsMonitors{
		gentype.NewClientWithList[*resourcestatemetricsv1alpha1.ResourceMetricsMonitor, *resourcestatemetricsv1alpha1.ResourceMetricsMonitorList](
			"resourcemetricsmonitors",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *resourcestatemetricsv1alpha1.ResourceMetricsMonitor {
				return &resourcestatemetricsv1alpha1.ResourceMetricsMonitor{}
			},
			func() *resourcestatemetricsv1alpha1.ResourceMetricsMonitorList {
				return &resourcestatemetricsv1alpha1.ResourceMetricsMonitorList{}
			},
		),
	}
}
