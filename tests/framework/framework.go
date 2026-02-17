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
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rexagod/resource-state-metrics/internal"
	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	rsmclientset "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned"
	rsmfake "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned/fake"
	"gopkg.in/yaml.v3"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	shortTimeInterval = 100 * time.Millisecond
	longTimeInterval  = time.Second
	gvkIndexName      = "gvk"
)

var (
	rmmGVR = schema.GroupVersionResource{
		Group:    v1alpha1.SchemeGroupVersion.Group,
		Version:  v1alpha1.SchemeGroupVersion.Version,
		Resource: "resourcemetricsmonitors",
	}
)

// Framework provides utilities for e2e testing with mock clientsets.
type Framework struct {
	KubeClient          kubernetes.Interface
	RSMClient           rsmclientset.Interface
	DynamicClient       *dynamicfake.FakeDynamicClient
	APIExtensionsClient apiextensionsclientset.Interface
	Controller          *internal.Controller
	Options             *internal.Options
	scheme              *runtime.Scheme
	crdInformer         cache.SharedIndexInformer
	crdInformerFactory  apiextensionsinformers.SharedInformerFactory
}

// New creates a new test framework with mock clientsets.
func New(ctx context.Context, addersToScheme ...func(*runtime.Scheme) error) *Framework {
	scheme := runtime.NewScheme()
	for _, adder := range addersToScheme {
		if err := adder(scheme); err != nil {
			panic(fmt.Sprintf("failed to add to scheme: %v", err))
		}
	}

	apiExtensionsClient := apiextensionsfake.NewSimpleClientset()
	crdInformerFactory := apiextensionsinformers.NewSharedInformerFactory(apiExtensionsClient, 0)
	crdInformer := crdInformerFactory.Apiextensions().V1().CustomResourceDefinitions().Informer()
	_ = crdInformer.AddIndexers(cache.Indexers{
		gvkIndexName: func(obj any) ([]string, error) {
			crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
			if !ok {
				return nil, errors.New("object is not a CRD")
			}
			var keys []string
			for _, version := range crd.Spec.Versions {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: version.Name,
					Kind:    crd.Spec.Names.Kind,
				}
				keys = append(keys, gvk.String())
			}

			return keys, nil
		},
	})

	f := &Framework{
		KubeClient:          kubefake.NewClientset(),
		RSMClient:           rsmfake.NewSimpleClientset(),
		DynamicClient:       dynamicfake.NewSimpleDynamicClient(scheme),
		APIExtensionsClient: apiExtensionsClient,
		scheme:              scheme,
		crdInformer:         crdInformer,
		crdInformerFactory:  crdInformerFactory,
	}

	crdInformerFactory.Start(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), crdInformer.HasSynced)

	return f
}

// Start starts the RSM controller with the mock clients.
func (f *Framework) Start(ctx context.Context, workers int) error {
	if f.Controller != nil {
		// Controller is already running
		return nil
	}

	f.Options = &internal.Options{Workers: &workers}
	f.Options.Read()

	f.Controller = internal.NewController(ctx, f.Options, f.KubeClient, f.RSMClient, f.DynamicClient)

	// Start controller in background
	go func() {
		if err := f.Controller.Run(ctx, *f.Options.Workers); err != nil {
			klog.FromContext(ctx).Error(err, "controller failed to start")
		}
	}()

	if err := f.waitForControllerReady(); err != nil {
		return fmt.Errorf("controller failed to become ready: %w", err)
	}

	return nil
}

// waitForControllerReady waits for the controller to be ready.
func (f *Framework) waitForControllerReady() error {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(shortTimeInterval)
	defer ticker.Stop()

	for {
		port := *f.Options.MainPort
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for controller main server port %d to open", port)
		case <-ticker.C:
			addr := fmt.Sprintf("127.0.0.1:%d", port)
			conn, err := net.DialTimeout("tcp", addr, 5*shortTimeInterval)
			if err == nil {
				_ = conn.Close()

				return nil
			}
		}
	}
}

// ApplyCRFromYAML applies a custom resource from a YAML file.
func (f *Framework) ApplyCRFromYAML(ctx context.Context, path string) (*unstructured.Unstructured, error) {
	data, err := os.ReadFile(ensureSafePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %w", path, err)
	}

	cr := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, cr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return f.ApplyCRUnstructured(ctx, cr)
}

// ApplyCRUnstructured applies a custom resource from an unstructured object.
func (f *Framework) ApplyCRUnstructured(ctx context.Context, customresource *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	gvk := customresource.GroupVersionKind()
	resource, err := f.GetResourcePluralNameForGVK(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource for %s: %w", gvk, err)
	}

	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource,
	}

	// Set default namespace if not specified
	if customresource.GetNamespace() == "" {
		customresource.SetNamespace("default")
	}

	resourceClient := f.DynamicClient.Resource(gvr).Namespace(customresource.GetNamespace())
	created, err := resourceClient.Create(ctx, customresource, metav1.CreateOptions{})
	if err == nil {
		return created, nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create CR %s/%s: %w", customresource.GetNamespace(), customresource.GetName(), err)
	}
	existing, err := resourceClient.Get(ctx, customresource.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get existing CR %s/%s: %w", customresource.GetNamespace(), customresource.GetName(), err)
	}

	customresource.SetResourceVersion(existing.GetResourceVersion())
	updated, err := resourceClient.Update(ctx, customresource, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update CR %s/%s: %w", customresource.GetNamespace(), customresource.GetName(), err)
	}

	return updated, nil
}

// GetCRUnstructured retrieves a custom resource as an unstructured object.
func (f *Framework) GetCRUnstructured(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	return f.DynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListCRsUnstructured lists custom resources as unstructured objects.
func (f *Framework) ListCRsUnstructured(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	return f.DynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// DeleteCR deletes a custom resource.
func (f *Framework) DeleteCR(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	err := f.DynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete CR %s/%s: %w", namespace, name, err)
	}

	return nil
}

// CreateCRDFromYAML creates a CRD from a YAML file and waits for it to be indexed.
func (f *Framework) CreateCRDFromYAML(ctx context.Context, path string) (*apiextensionsv1.CustomResourceDefinition, error) {
	data, err := os.ReadFile(ensureSafePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %w", path, err)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(data, crd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	created, err := f.APIExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	if err := f.waitForCRDIndexed(created); err != nil {
		return nil, fmt.Errorf("CRD created but failed to index: %w", err)
	}

	return created, nil
}

// GetResourcePluralNameForGVK returns the plural resource name for a given GVK by querying the CRD informer index.
func (f *Framework) GetResourcePluralNameForGVK(gvk schema.GroupVersionKind) (string, error) {
	objs, err := f.crdInformer.GetIndexer().ByIndex(gvkIndexName, gvk.String())
	if err != nil {
		return "", fmt.Errorf("failed to query CRD index for %s: %w", gvk.String(), err)
	}

	if len(objs) == 0 {
		return "", fmt.Errorf("no CRD found for %s", gvk.String())
	}

	crd, ok := objs[0].(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		return "", fmt.Errorf("unexpected type in CRD index for %s: %T", gvk.String(), objs[0])
	}

	return crd.Spec.Names.Plural, nil
}

// ToUnstructured converts a runtime.Object to an unstructured.Unstructured.
func (f *Framework) ToUnstructured(o runtime.Object) (*unstructured.Unstructured, error) {
	stringToInterfaceMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}
	unstructuredObj := &unstructured.Unstructured{Object: stringToInterfaceMap}
	unstructuredObj.SetGroupVersionKind(o.GetObjectKind().GroupVersionKind())

	return unstructuredObj, nil
}

// FromUnstructured converts an unstructured.Unstructured back to a runtime.Object (populates the supplied object).
func (f *Framework) FromUnstructured(u *unstructured.Unstructured, o runtime.Object) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, o)
}

// waitForCRDIndexed waits for a CRD to appear in the informer index.
func (f *Framework) waitForCRDIndexed(crd *apiextensionsv1.CustomResourceDefinition) error {
	timeout := time.After(longTimeInterval)
	ticker := time.NewTicker(shortTimeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out while waiting for CRD (%s) to be indexed", crd.Name)
		case <-ticker.C:
			for _, version := range crd.Spec.Versions {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: version.Name,
					Kind:    crd.Spec.Names.Kind,
				}
				objs, err := f.crdInformer.GetIndexer().ByIndex(gvkIndexName, gvk.String())
				if err == nil && len(objs) > 0 {
					return nil
				}
			}
		}
	}
}

// CRBuilder helps build custom resources.
type CRBuilder struct {
	cr *unstructured.Unstructured
}

// NewCRBuilder returns a builder for constructing unstructured CRs.
func NewCRBuilder(group, version, kind, name, namespace string) *CRBuilder {
	cr := &unstructured.Unstructured{
		Object: make(map[string]any),
	}
	cr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})
	cr.SetName(name)
	cr.SetNamespace(namespace)

	return &CRBuilder{cr: cr}
}

// WithSpec sets a field in the spec.
// Panics if the field cannot be set.
func (b *CRBuilder) WithSpec(path string, value any) *CRBuilder {
	// Convert int to int64 for JSON compatibility
	switch v := value.(type) {
	case int:
		value = int64(v)
	case int32:
		value = int64(v)
	}

	if err := unstructured.SetNestedField(b.cr.Object, value, "spec", path); err != nil {
		panic(fmt.Sprintf("failed to set spec field %q: %v", path, err))
	}

	return b
}

// WithLabel adds a label.
func (b *CRBuilder) WithLabel(key, value string) *CRBuilder {
	labels := b.cr.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	b.cr.SetLabels(labels)

	return b
}

// WithAnnotation adds an annotation.
func (b *CRBuilder) WithAnnotation(key, value string) *CRBuilder {
	annotations := b.cr.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	b.cr.SetAnnotations(annotations)

	return b
}

// Build returns the constructed unstructured CR.
func (b *CRBuilder) Build() *unstructured.Unstructured {
	return b.cr
}

// ensureSafePath checks if the provided path is within the tests directory to prevent file system access outside of the intended scope.
func ensureSafePath(path string) string {
	cleanedPath := filepath.Clean(path)
	absolutePath, err := filepath.Abs(cleanedPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to get absolute path: %v", err))
	}
	testsDir, err := filepath.Abs("..")
	if err != nil {
		panic(fmt.Sprintf("Failed to get absolute path of tests directory: %v", err))
	}
	if !strings.HasPrefix(absolutePath, testsDir) {
		panic(fmt.Sprintf("Unsafe path detected: %s is outside of the tests directory", absolutePath))
	}

	return absolutePath
}
