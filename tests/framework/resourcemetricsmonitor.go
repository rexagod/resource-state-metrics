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

package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplyRMM applies a ResourceMetricsMonitor resource using ApplyCR.
func (f *Framework) ApplyRMM(ctx context.Context, rmm *v1alpha1.ResourceMetricsMonitor) (*v1alpha1.ResourceMetricsMonitor, error) {
	cr, err := f.ToUnstructured(rmm)
	if err != nil {
		return nil, fmt.Errorf("failed to convert RMM to unstructured: %w", err)
	}

	appliedCR, err := f.ApplyCRUnstructured(ctx, cr)
	if err != nil {
		return nil, err
	}

	obj := &v1alpha1.ResourceMetricsMonitor{}
	err = f.FromUnstructured(appliedCR, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert applied CR to RMM: %w", err)
	}

	return obj, nil
}

// ApplyRMMFromYAML applies a ResourceMetricsMonitor resource from a YAML file using ApplyCRFromYAML.
func (f *Framework) ApplyRMMFromYAML(ctx context.Context, path string) (*v1alpha1.ResourceMetricsMonitor, error) {
	appliedCR, err := f.ApplyCRFromYAML(ctx, path)
	if err != nil {
		return nil, err
	}

	obj := &v1alpha1.ResourceMetricsMonitor{}
	err = f.FromUnstructured(appliedCR, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert applied CR to RMM: %w", err)
	}

	return obj, nil
}

// WaitForRMMProcessed waits for an RMM to be processed (status condition set).
func (f *Framework) WaitForRMMProcessed(ctx context.Context, namespace, name string, timeout time.Duration) (*v1alpha1.ResourceMetricsMonitor, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(shortTimeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			rmm, err := f.RSMClient.ResourceStateMetricsV1alpha1().ResourceMetricsMonitors(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			for _, cond := range rmm.Status.Conditions {
				if cond.Type == v1alpha1.ConditionType[v1alpha1.ConditionTypeProcessed] {
					return rmm, nil
				}
			}
		}
	}
}

// DeleteRMM deletes a ResourceMetricsMonitor using DeleteCR.
func (f *Framework) DeleteRMM(ctx context.Context, namespace, name string) error {
	return f.DeleteCR(ctx, rmmGVR, namespace, name)
}
