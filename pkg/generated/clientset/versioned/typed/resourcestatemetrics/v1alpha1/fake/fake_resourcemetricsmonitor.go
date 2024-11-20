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

package fake

import (
	"context"

	v1alpha1 "github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeResourceMetricsMonitors implements ResourceMetricsMonitorInterface
type FakeResourceMetricsMonitors struct {
	Fake *FakeResourceStateMetricsV1alpha1
	ns   string
}

var resourcemetricsmonitorsResource = v1alpha1.SchemeGroupVersion.WithResource("resourcemetricsmonitors")

var resourcemetricsmonitorsKind = v1alpha1.SchemeGroupVersion.WithKind("ResourceMetricsMonitor")

// Get takes name of the resourceMetricsMonitor, and returns the corresponding resourceMetricsMonitor object, and an error if there is any.
func (c *FakeResourceMetricsMonitors) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ResourceMetricsMonitor, err error) {
	emptyResult := &v1alpha1.ResourceMetricsMonitor{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(resourcemetricsmonitorsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ResourceMetricsMonitor), err
}

// List takes label and field selectors, and returns the list of ResourceMetricsMonitors that match those selectors.
func (c *FakeResourceMetricsMonitors) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ResourceMetricsMonitorList, err error) {
	emptyResult := &v1alpha1.ResourceMetricsMonitorList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(resourcemetricsmonitorsResource, resourcemetricsmonitorsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ResourceMetricsMonitorList{ListMeta: obj.(*v1alpha1.ResourceMetricsMonitorList).ListMeta}
	for _, item := range obj.(*v1alpha1.ResourceMetricsMonitorList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested resourceMetricsMonitors.
func (c *FakeResourceMetricsMonitors) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(resourcemetricsmonitorsResource, c.ns, opts))

}

// Create takes the representation of a resourceMetricsMonitor and creates it.  Returns the server's representation of the resourceMetricsMonitor, and an error, if there is any.
func (c *FakeResourceMetricsMonitors) Create(ctx context.Context, resourceMetricsMonitor *v1alpha1.ResourceMetricsMonitor, opts v1.CreateOptions) (result *v1alpha1.ResourceMetricsMonitor, err error) {
	emptyResult := &v1alpha1.ResourceMetricsMonitor{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(resourcemetricsmonitorsResource, c.ns, resourceMetricsMonitor, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ResourceMetricsMonitor), err
}

// Update takes the representation of a resourceMetricsMonitor and updates it. Returns the server's representation of the resourceMetricsMonitor, and an error, if there is any.
func (c *FakeResourceMetricsMonitors) Update(ctx context.Context, resourceMetricsMonitor *v1alpha1.ResourceMetricsMonitor, opts v1.UpdateOptions) (result *v1alpha1.ResourceMetricsMonitor, err error) {
	emptyResult := &v1alpha1.ResourceMetricsMonitor{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(resourcemetricsmonitorsResource, c.ns, resourceMetricsMonitor, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ResourceMetricsMonitor), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeResourceMetricsMonitors) UpdateStatus(ctx context.Context, resourceMetricsMonitor *v1alpha1.ResourceMetricsMonitor, opts v1.UpdateOptions) (result *v1alpha1.ResourceMetricsMonitor, err error) {
	emptyResult := &v1alpha1.ResourceMetricsMonitor{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(resourcemetricsmonitorsResource, "status", c.ns, resourceMetricsMonitor, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ResourceMetricsMonitor), err
}

// Delete takes name of the resourceMetricsMonitor and deletes it. Returns an error if one occurs.
func (c *FakeResourceMetricsMonitors) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(resourcemetricsmonitorsResource, c.ns, name, opts), &v1alpha1.ResourceMetricsMonitor{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeResourceMetricsMonitors) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(resourcemetricsmonitorsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.ResourceMetricsMonitorList{})
	return err
}

// Patch applies the patch and returns the patched resourceMetricsMonitor.
func (c *FakeResourceMetricsMonitors) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ResourceMetricsMonitor, err error) {
	emptyResult := &v1alpha1.ResourceMetricsMonitor{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(resourcemetricsmonitorsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.ResourceMetricsMonitor), err
}