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
	stderrors "errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/rexagod/resource-state-metrics/internal/version"
	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	clientset "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type eventType int

const (
	addEvent eventType = iota
	updateEvent
	deleteEvent
)

func (e eventType) String() string {
	return []string{"addEvent", "updateEvent", "deleteEvent"}[e]
}

type handler struct {
	kubeClientset    kubernetes.Interface
	rsmClientset     clientset.Interface
	dynamicClientset dynamic.Interface
	celCostLimit     uint64
	celTimeout       time.Duration
}

func newHandler(kubeClientset kubernetes.Interface, rsmClientset clientset.Interface, dynamicClientset dynamic.Interface, celCostLimit uint64, celTimeout time.Duration) *handler {
	return &handler{
		kubeClientset:    kubeClientset,
		rsmClientset:     rsmClientset,
		dynamicClientset: dynamicClientset,
		celCostLimit:     celCostLimit,
		celTimeout:       celTimeout,
	}
}

func (h *handler) handleEvent(ctx context.Context, stores *sync.Map, event string, o metav1.Object) error {
	logger := klog.FromContext(ctx)

	resource, ok := o.(*v1alpha1.ResourceMetricsMonitor)
	if !ok {
		logger.Error(fmt.Errorf("failed to cast object to %s", resource.GetObjectKind()), "cannot handle event")

		return nil
	}
	kObj := klog.KObj(resource).String()

	if err := h.updateMetadata(ctx, resource); err != nil {
		logger.Error(fmt.Errorf("failed to update metadata for %s: %w", kObj, err), "cannot handle event")

		return nil
	}

	updatedResource, err := h.emitSuccess(ctx, resource, metav1.ConditionFalse, fmt.Sprintf("Event handler received event: %s", event))
	if err != nil {
		logger.Error(fmt.Errorf("failed to emit success on %s: %w", kObj, err), "cannot update the resource")

		return nil
	}
	resource = updatedResource

	if resource.Spec.Configuration == "" {
		logger.Error(stderrors.New("configuration YAML is empty"), "cannot process the resource")
		h.emitFailure(ctx, resource, "Configuration YAML is empty")

		return nil
	}

	configurerInstance := newConfigurer(h.dynamicClientset, resource, h.celCostLimit, h.celTimeout)
	dropStores := func() {
		stores.Delete(resource.GetUID())
	}

	switch event {
	case addEvent.String(), updateEvent.String():
		dropStores()
		if err := configurerInstance.parse(resource.Spec.Configuration); err != nil {
			logger.Error(fmt.Errorf("failed to parse configuration YAML: %w", err), "cannot process the resource")
			h.emitFailure(ctx, resource, fmt.Sprintf("Failed to parse configuration YAML: %s", err))

			return nil
		}
		configurerInstance.build(ctx, stores)

	case deleteEvent.String():
		dropStores()

	default:
		logger.Error(fmt.Errorf("unknown event type (%s)", event), "cannot process the resource")
		h.emitFailure(ctx, resource, fmt.Sprintf("Unknown event type: %s", event))

		return nil
	}

	if _, err := h.emitSuccess(ctx, resource, metav1.ConditionTrue, fmt.Sprintf("Event handler successfully processed event: %s", event)); err != nil {
		logger.Error(fmt.Errorf("failed to emit success on %s: %w", kObj, err), "cannot update the resource")

		return nil
	}

	return nil
}

func (h *handler) emitSuccess(ctx context.Context, monitor *v1alpha1.ResourceMetricsMonitor, statusBool metav1.ConditionStatus, message string) (*v1alpha1.ResourceMetricsMonitor, error) {
	kObj := klog.KObj(monitor).String()

	resource, err := h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(monitor.GetNamespace()).
		Get(ctx, monitor.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", kObj, err)
	}
	resource.Status.Set(resource, metav1.Condition{
		Type:    v1alpha1.ConditionType[v1alpha1.ConditionTypeProcessed],
		Status:  statusBool,
		Message: message,
	})
	resource, err = h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).
		UpdateStatus(ctx, resource, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update the status of %s: %w", kObj, err)
	}

	return resource, nil
}

func (h *handler) emitFailure(ctx context.Context, monitor *v1alpha1.ResourceMetricsMonitor, message string) {
	kObj := klog.KObj(monitor).String()

	resource, err := h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(monitor.GetNamespace()).
		Get(ctx, monitor.GetName(), metav1.GetOptions{})
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
	}
}

func (h *handler) updateMetadata(ctx context.Context, resource *v1alpha1.ResourceMetricsMonitor) error {
	logger := klog.FromContext(ctx)
	kObj := klog.KObj(resource).String()

	return wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, false, func(pollCtx context.Context) (bool, error) {
		gotResource, err := h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).Get(pollCtx, resource.GetName(), metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get %s: %w", kObj, err)
		}
		resource = gotResource.DeepCopy()

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

		resource, err = h.rsmClientset.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(resource.GetNamespace()).Update(pollCtx, resource, metav1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to update %s: %w", kObj, err)
		}

		return true, nil
	})
}
