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

/*
This test performs conformance testing for ResourceMetricsMonitor behavior as
seen with the ResourceStateMetrics controller. It does so by testing all golden
rules defined under each resolver's "customresourcestatemetrics_conformance"
directory.

It verifies feature parity with KubeStateMetrics' CustomResourceStateMetrics
feature-set, by deploying a set of golden ResourceMetricsMonitor
configurations, each reflecting an existing KubeStateMetrics'
CustomResourceStateMetrics configuration, and validating that:
* there are no errors, and,
* the expected metrics are emitted with the expected labelsets.

Certain behaviors may differ under the ResourceStateMetrics controller, owing
to them simply making more sense generally, and will be documented in their
respective golden configuration files.
*/

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rexagod/resource-state-metrics/internal"
	"github.com/rexagod/resource-state-metrics/tests/framework"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestCustomResourceStateMetricsConformance tests all golden rules for all resolvers.
func TestCustomResourceStateMetricsConformance(t *testing.T) {
	ctx := context.Background()

	// Pre-load RMMs from golden rules to work around the fact that fake clients
	// don't emit watch events for objects created after informers start, so RMMs
	// must be pre-populated before the controller's informer initializes.
	initialRMMs, err := framework.LoadRMMsFromGoldenRules(ctx)
	if err != nil {
		t.Fatalf("Failed to load RMMs from golden rules: %v", err)
	}

	f := framework.NewInforming(ctx, initialRMMs...)

	if err := applyCRDManifests(t, ctx, f); err != nil {
		t.Fatalf("Failed to apply CRD manifests: %v", err)
	}

	gvrToKindListMap := make(map[schema.GroupVersionResource]string)
	indexedCRDs := f.GetIndexedCRDs()

	for _, crd := range indexedCRDs {
		for _, version := range crd.Spec.Versions {
			gv := schema.GroupVersion{Group: crd.Spec.Group, Version: version.Name}

			f.AddToScheme(func(scheme *runtime.Scheme) {
				scheme.AddKnownTypes(gv, &unstructured.Unstructured{}, &unstructured.UnstructuredList{})
			})

			// The dynamic client needs to know the List kind for each GVR to
			// properly handle list operations. This is typically the singular Kind
			// with "List" appended. This is also the reason why we aren't just
			// passing the updated scheme to the dynamic client, as it doesn't have
			// the necessary type information to derive the List kinds on its own.
			// Regardless, we still update the scheme for other clients that may need it.
			gvr := schema.GroupVersionResource{
				Group:    crd.Spec.Group,
				Version:  version.Name,
				Resource: crd.Spec.Names.Plural,
			}
			gvrToKindListMap[gvr] = crd.Spec.Names.Kind + "List"
		}
	}

	f.WithDynamicClient(gvrToKindListMap)

	if err := applyCRManifests(t, ctx, f); err != nil {
		t.Fatalf("Failed to apply CR manifests: %v", err)
	}

	if err := f.Start(ctx, 1); err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}

	for _, resolverType := range []internal.ResolverType{
		internal.ResolverTypeUnstructured,
	} {
		t.Run(string(resolverType), func(t *testing.T) {
			testResolverConformance(t, ctx, f, resolverType)
		})
	}
}

// getCRDandNonCRDManifests retrieves all CRD and non-CRD manifest file paths from the specified directories.
func getCRDandNonCRDManifests(t *testing.T) ([]string, []string, error) {
	manifestDirs := []string{
		"manifests",
		"../manifests",
	}

	// Fake client does not support certain resources OOTB.
	ignoredManifestsByPrefix := map[string]struct{}{
		"cluster-role": {},
	}

	var (
		crdFiles   []string
		otherFiles []string
	)

	for _, manifestsDir := range manifestDirs {
		if _, err := os.Stat(manifestsDir); os.IsNotExist(err) {
			t.Fatalf("Manifests directory does not exist: %s", manifestsDir)
		}

		err := filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			for ignoredPrefix := range ignoredManifestsByPrefix {
				if strings.HasPrefix(filepath.Base(path), ignoredPrefix) {
					return nil
				}
			}

			if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
				return nil
			}

			// Assume all CRD manifests are prefixed with "custom-resource-definition"
			if strings.HasPrefix(filepath.Base(path), "custom-resource-definition") {
				crdFiles = append(crdFiles, path)
			} else {
				otherFiles = append(otherFiles, path)
			}

			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	return crdFiles, otherFiles, nil
}

// applyCRDManifests applies only CRD manifests from the manifest directories.
func applyCRDManifests(t *testing.T, ctx context.Context, f *framework.Framework) error {
	crdFiles, _, err := getCRDandNonCRDManifests(t)
	if err != nil {
		return fmt.Errorf("failed to get manifest files: %w", err)
	}

	for _, path := range crdFiles {
		if _, err := f.CreateCRDFromYAML(ctx, path); err != nil {
			return fmt.Errorf("failed to create CRD from %s: %w", path, err)
		}
	}

	return nil
}

// applyCRManifests applies only CR manifests (non-CRD) from the manifest directories.
func applyCRManifests(t *testing.T, ctx context.Context, f *framework.Framework) error {
	_, otherFiles, err := getCRDandNonCRDManifests(t)
	if err != nil {
		return fmt.Errorf("failed to get manifest files: %w", err)
	}

	for _, path := range otherFiles {
		if _, err := f.ApplyCRFromYAML(ctx, path); err != nil {
			return fmt.Errorf("failed to apply CR from %s: %w", path, err)
		}
	}

	return nil
}

// testResolverConformance tests all golden rules for a specific resolver.
func testResolverConformance(t *testing.T, ctx context.Context, f *framework.Framework, resolverType internal.ResolverType) {
	files := framework.GetConformanceGoldenRuleFiles([]internal.ResolverType{resolverType})

	if len(files) == 0 {
		t.Fatalf("No golden rule files found")
		return
	}

	for _, file := range files {
		testName := strings.TrimSuffix(filepath.Base(file), ".yaml")
		t.Run(testName, func(t *testing.T) {
			testGoldenRule(t, ctx, f, file)
		})
	}
}

// testGoldenRule tests a single golden rule file.
func testGoldenRule(t *testing.T, ctx context.Context, f *framework.Framework, filePath string) {
	goldenRule, err := framework.GoldenRuleFromYAML(ctx, filePath)
	if err != nil {
		t.Fatalf("Failed to load golden rule from %s: %v", filePath, err)
	}

	if goldenRule.In == nil {
		t.Skipf("Golden rule has no input resource defined, skipping")
		return
	}

	// RMMs are pre-loaded when creating the framework, so only apply non-RMM resources
	if goldenRule.In != nil && goldenRule.In.GetKind() != framework.ResourceMetricsMonitorKind {
		_, err := f.ApplyCRUnstructured(ctx, goldenRule.In)
		if err != nil {
			t.Fatalf("Failed to apply input resource: %v", err)
		}
	}

	// Wait for controller to process resources and reflectors to sync
	time.Sleep(5 * framework.LongTimeInterval)

	goldenRuleOutMetrics := goldenRule.Out.Metrics
	if len(goldenRuleOutMetrics) == 0 {
		panic("Golden rule has no expected output metrics defined")
	}

	// Extract metric names for filtering
	// TODO
	var metricNames []string
	for _, line := range goldenRuleOutMetrics {
		if strings.HasPrefix(line, "# ") {
			continue
		}
		parts := strings.SplitN(line, "{", 2)
		if len(parts) > 0 {
			metricName := strings.TrimSpace(parts[0])
			if metricName != "" && !slices.Contains(metricNames, metricName) {
				metricNames = append(metricNames, metricName)
			}
		}
	}

	expectedMetrics := strings.Join(goldenRuleOutMetrics, "\n") + "\n"
	port := *f.Options.MainPort
	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)

	if err := testutil.ScrapeAndCompare(url, strings.NewReader(expectedMetrics), metricNames...); err != nil {
		t.Errorf("Metric comparison failed: %v", err)
		return
	}
}
