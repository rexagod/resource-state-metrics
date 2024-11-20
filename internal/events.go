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
	stderrors "errors"
	"fmt"
	"regexp"
	"time"

	"github.com/rexagod/resource-state-metrics/internal/version"
	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	clientset "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// eventType represents the type of event received from the informer.
type eventType int

const (
	addEvent eventType = iota
	updateEvent
	deleteEvent
)

func (e eventType) String() string {
	return []string{"addEvent", "updateEvent", "deleteEvent"}[e]
}

// handler knows how to handle resource events.
type handler struct {

	// kubeClientset is the clientset used to interact with the Kubernetes API.
	kubeClientset kubernetes.Interface

	// rsmClientset is the clientset used to update the status of the managed resource.
	rsmClientset clientset.Interface

	// dynamicClientset is the dynamic clientset used to build stores for different objects.
	dynamicClientset dynamic.Interface
}

// newHandler creates a new handler.
func newHandler(
	kubeClientset kubernetes.Interface,
	rsmClientset clientset.Interface,
	dynamicClientset dynamic.Interface,
) *handler {
	return &handler{
		kubeClientset:    kubeClientset,
		rsmClientset:     rsmClientset,
		dynamicClientset: dynamicClientset,
	}
}

// HandleEvent handles events received from the informer.
func (h *handler) handleEvent(
	ctx context.Context,
	uidToStoresMap map[types.UID][]*StoreType,
	event string,
	o metav1.Object,
	tryNoCache bool,
) error {
	logger := klog.FromContext(ctx)

	// Resolve the object type.
	resource, ok := o.(*v1alpha1.ResourceMetricsMonitor)
	if !ok {
		logger.Error(fmt.Errorf("failed to cast object to %s", resource.GetObjectKind()), "cannot handle event")

		return nil // Do not requeue.
	}
	kObj := klog.KObj(resource).String()

	// Preemptively update the resource metadata. We poll here to avoid same resource versions across update bursts.
	err := h.updateMetadata(ctx, resource)
	if err != nil {
		logger.Error(fmt.Errorf("failed to update metadata for %s: %w", kObj, err), "cannot handle event")

		return nil // Do not requeue.
	}

	// Update resource status.
	resource, err = h.emitSuccessOnResource(ctx, resource, metav1.ConditionFalse, fmt.Sprintf("Event handler received event: %s", event))
	if err != nil {
		logger.Error(fmt.Errorf("failed to emit success on %s: %w", kObj, err), "cannot update the resource")

		return nil // Do not requeue.
	}

	// Process the fetched configuration.
	configurationYAML := resource.Spec.Configuration
	if configurationYAML == "" {
		// This should never happen owing to the Kubebuilder check in place.
		logger.Error(stderrors.New("configuration YAML is empty"), "cannot process the resource")
		h.emitFailureOnResource(ctx, resource, "Configuration YAML is empty")

		return nil
	}
	configurerInstance := newConfigurer(h.dynamicClientset, resource)

	// dropStores drops associated stores between resource changes.
	dropStores := func() {
		resourceUID := resource.GetUID()
		if _, ok = uidToStoresMap[resourceUID]; ok {
			// The associated stores are only reachable through the map. Deleting them will trigger the GC.
			delete(uidToStoresMap, resourceUID)
		}
	}

	// Handle the event.
	switch event {
	// Build all associated stores.
	case addEvent.String(), updateEvent.String():
		dropStores()
		err = configurerInstance.parse(configurationYAML)
		if err != nil {
			logger.Error(fmt.Errorf("failed to parse configuration YAML: %w", err), "cannot process the resource")
			h.emitFailureOnResource(ctx, resource, fmt.Sprintf("Failed to parse configuration YAML: %s", err))

			return nil
		}
		configurerInstance.build(ctx, uidToStoresMap, tryNoCache)

	// Drop all associated stores.
	case deleteEvent.String():
		dropStores()

	// This should never happen.
	default:
		logger.Error(fmt.Errorf("unknown event type (%s)", event), "cannot process the resource")
		h.emitFailureOnResource(ctx, resource, fmt.Sprintf("Unknown event type: %s", event))

		return nil
	}

	// Update the status of the resource.
	_, err = h.emitSuccessOnResource(ctx, resource, metav1.ConditionTrue, fmt.Sprintf("Event handler successfully processed event: %s", event))
	if err != nil {
		logger.Error(fmt.Errorf("failed to emit success on %s: %w", kObj, err), "cannot update the resource")

		return nil // Do not requeue.
	}

	return nil
}

// emitSuccessOnResource emits a success condition on the given resource.
func (h *handler) emitSuccessOnResource(
	ctx context.Context,
	gotResource *v1alpha1.ResourceMetricsMonitor,
	conditionBool metav1.ConditionStatus,
	message string,
) (*v1alpha1.ResourceMetricsMonitor, error) {
	kObj := klog.KObj(gotResource).String()

	resource, err := h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(gotResource.GetNamespace()).
		Get(ctx, gotResource.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", kObj, err)
	}
	resource.Status.Set(resource, metav1.Condition{
		Type:    v1alpha1.ConditionType[v1alpha1.ConditionTypeProcessed],
		Status:  conditionBool,
		Message: message,
	})
	resource, err = h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).
		UpdateStatus(ctx, resource, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update the status of %s: %w", kObj, err)
	}

	return resource, nil
}

// emitFailureOnResource emits a failure condition on the given resource.
func (h *handler) emitFailureOnResource(
	ctx context.Context,
	gotResource *v1alpha1.ResourceMetricsMonitor,
	message string,
) /* Don't return the most recent resource since this call should always precede an empty return. */ {
	kObj := klog.KObj(gotResource).String()

	resource, err := h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(gotResource.GetNamespace()).
		Get(ctx, gotResource.GetName(), metav1.GetOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to get %s: %w", kObj, err))

		return
	}
	resource.Status.Set(resource, metav1.Condition{
		Type:    v1alpha1.ConditionType[v1alpha1.ConditionTypeFailed],
		Status:  metav1.ConditionTrue,
		Message: message,
	})
	_, err = h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).
		UpdateStatus(ctx, resource, metav1.UpdateOptions{})
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to emit failure on %s: %w", kObj, err))

		return
	}
}

// updateMetadata updates the metadata of the managed resource.
func (h *handler) updateMetadata(ctx context.Context, resource *v1alpha1.ResourceMetricsMonitor) error {
	logger := klog.FromContext(ctx)
	kObj := klog.KObj(resource).String()

	err := wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, false, func(context.Context) (
		bool,
		error,
	) {
		gotResource, err := h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).
			Get(ctx, resource.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get %s: %w", kObj, err)
		}
		resource = gotResource.DeepCopy() // Ensure we are working with the latest resourceVersion.

		// Add relevant metadata information to the resource.
		// Build relevant labels.
		if resource.Labels == nil {
			resource.Labels = make(map[string]string)
		}
		resource.Labels["app.kubernetes.io/managed-by"] = version.ControllerName.String()
		revisionSHA := regexp.MustCompile(`revision:\s*(\S+)\)`).FindStringSubmatch(version.Version())
		if len(revisionSHA) > 1 {
			resource.Labels["app.kubernetes.io/version"] = revisionSHA[1]
		} else {
			logger.Error(stderrors.New("failed to get revision SHA, continuing anyway"), "cannot set version label")
		}

		// Compare resource with the fetched resource.
		resource, err = h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).
			Update(ctx, resource, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update %s: %w", kObj, err)
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("failed while polling for %s: %w", kObj, err)
	}

	return nil
}
