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
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rexagod/resource-state-metrics/internal/version"
	"github.com/rexagod/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	clientset "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned"
	rsmscheme "github.com/rexagod/resource-state-metrics/pkg/generated/clientset/versioned/scheme"
	informers "github.com/rexagod/resource-state-metrics/pkg/generated/informers/externalversions"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type metrics struct {
	requestDurationVec *prometheus.HistogramVec
	resourcesMonitored *prometheus.GaugeVec
	eventsProcessed    *prometheus.CounterVec
	configParseErrors  *prometheus.CounterVec
	celEvaluations     *prometheus.CounterVec
}

// Controller is the controller implementation for managed resources.
type Controller struct {
	kubeclientset      kubernetes.Interface
	rsmClientset       clientset.Interface
	dynamicClientset   dynamic.Interface
	rsmInformerFactory informers.SharedInformerFactory
	workqueue          workqueue.TypedRateLimitingInterface[[2]string]
	recorder           record.EventRecorder
	stores             sync.Map
	options            *Options

	metrics
}

// NewController returns a new controller instance.
func NewController(ctx context.Context, options *Options, kubeClientset kubernetes.Interface, rsmClientset clientset.Interface, dynamicClientset dynamic.Interface) *Controller {
	logger := klog.FromContext(ctx)
	utilruntime.Must(rsmscheme.AddToScheme(scheme.Scheme))

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeClientset.CoreV1().Events(metav1.NamespaceNone),
	})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: version.ControllerName.String()})

	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[[2]string](5*time.Millisecond, 5*time.Minute),
		&workqueue.TypedBucketRateLimiter[[2]string]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &Controller{
		kubeclientset:      kubeClientset,
		rsmClientset:       rsmClientset,
		dynamicClientset:   dynamicClientset,
		rsmInformerFactory: informers.NewSharedInformerFactory(rsmClientset, 0),
		workqueue:          workqueue.NewTypedRateLimitingQueue[[2]string](ratelimiter),
		recorder:           recorder,
		options:            options,
	}

	controller.registerEventHandlers(logger)

	return controller
}

// Run starts the controller and blocks until the context is cancelled.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	logger := klog.FromContext(ctx)
	logger.V(1).Info("Starting controller")
	logger.V(4).Info("Waiting for informer caches to sync")

	c.rsmInformerFactory.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), c.rsmInformerFactory.ResourceStateMetrics().V1alpha1().ResourceMetricsMonitors().Informer().HasSynced); !ok {
		return stderrors.New("failed to wait for caches to sync")
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		versioncollector.NewCollector(version.ControllerName.ToSnakeCase()),
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{Namespace: version.ControllerName.ToSnakeCase(), ReportErrors: true}),
	)

	namespace := version.ControllerName.ToSnakeCase()
	c.requestDurationVec = promauto.With(registry).NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "http_request_duration_seconds",
		Help:      "A histogram of requests for the main server's metrics endpoint.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "code"})

	c.resourcesMonitored = promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "resources_monitored_info",
		Help:      "Information about ResourceMetricsMonitor resources currently being monitored.",
	}, []string{"namespace", "name"})

	c.eventsProcessed = promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "events_processed_total",
		Help:      "Total number of events processed by type and status.",
	}, []string{"namespace", "name", "event_type", "status"})

	c.configParseErrors = promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "config_parse_errors_total",
		Help:      "Total number of configuration parsing errors.",
	}, []string{"namespace", "name"})

	c.celEvaluations = promauto.With(registry).NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cel_evaluations_total",
		Help:      "Total number of CEL expression evaluations by result.",
	}, []string{"namespace", "name", "family", "result"})

	selfAddr := net.JoinHostPort(*c.options.SelfHost, strconv.Itoa(*c.options.SelfPort))
	mainAddr := net.JoinHostPort(*c.options.MainHost, strconv.Itoa(*c.options.MainPort))

	self := newSelfServer(selfAddr).build(ctx, c.kubeclientset, registry)
	main := newMainServer(mainAddr, *c.options.Kubeconfig, &c.stores, c.requestDurationVec).build(ctx, c.kubeclientset, registry)

	logger.V(1).Info("Starting workers")
	for range workers {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			for c.processNextWorkItem(ctx) {
			}
		}, time.Second)
	}

	go func() {
		logger.V(1).Info("Starting telemetry server on", "address", selfAddr)
		if err := self.ListenAndServe(); err != nil {
			logger.Error(err, "stopping telemetry server")
		}
	}()
	go func() {
		logger.V(1).Info("Starting main server on", "address", mainAddr)
		if err := main.ListenAndServe(); err != nil {
			logger.Error(err, "stopping main server")
		}
	}()

	<-ctx.Done()
	logger.V(1).Info("Shutting down servers")
	if err := self.Shutdown(ctx); err != nil {
		logger.Error(err, "error shutting down telemetry server")
	}
	if err := main.Shutdown(ctx); err != nil {
		logger.Error(err, "error shutting down main server")
	}

	return nil
}

func (c *Controller) registerEventHandlers(logger klog.Logger) {
	_, err := c.rsmInformerFactory.ResourceStateMetrics().V1alpha1().ResourceMetricsMonitors().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.enqueue(obj, addEvent) },
		UpdateFunc: c.updateHandler(logger),
		DeleteFunc: func(obj interface{}) { c.enqueue(obj, deleteEvent) },
	})
	if err != nil {
		logger.Error(err, "error setting up event handlers")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}

func (c *Controller) updateHandler(logger klog.Logger) func(interface{}, interface{}) {
	return func(oldI, newI interface{}) {
		oldResource, ok := oldI.(*v1alpha1.ResourceMetricsMonitor)
		if !ok {
			logger.Error(stderrors.New("failed to cast old object to ResourceMetricsMonitor"), "cannot handle update event")

			return
		}
		newResource, ok := newI.(*v1alpha1.ResourceMetricsMonitor)
		if !ok {
			logger.Error(stderrors.New("failed to cast new object to ResourceMetricsMonitor"), "cannot handle update event")

			return
		}
		if oldResource.ResourceVersion == newResource.ResourceVersion || reflect.DeepEqual(oldResource.Spec, newResource.Spec) {
			logger.V(10).Info("Skipping event", "[-old +new]", cmp.Diff(oldResource, newResource))

			return
		}
		logger.V(4).Info("Update event", "[-old +new]", cmp.Diff(oldResource.Spec.Configuration, newResource.Spec.Configuration))
		c.enqueue(newI, updateEvent)
	}
}

func (c *Controller) enqueue(obj interface{}, event eventType) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)

		return
	}
	c.workqueue.Add([2]string{key, event.String()})
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	logger := klog.FromContext(ctx)
	objectWithEvent, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	err := func(objectWithEvent [2]string) error {
		defer c.workqueue.Done(objectWithEvent)
		key := objectWithEvent[0]
		event := objectWithEvent[1]
		if err := c.syncHandler(ctx, key, event); err != nil {
			c.workqueue.AddRateLimited(objectWithEvent)

			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		c.workqueue.Forget(objectWithEvent)
		logger.V(4).Info("Synced", "key", key)

		return nil
	}(objectWithEvent)

	if err != nil {
		logger.Error(err, "error processing item")

		return true
	}

	return true
}

func (c *Controller) syncHandler(ctx context.Context, key string, event string) error {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Syncing", "key", key, "event", event)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Error(err, "invalid resource key", "key", key)

		return nil
	}
	resource, err := c.rsmInformerFactory.ResourceStateMetrics().V1alpha1().ResourceMetricsMonitors().Lister().ResourceMetricsMonitors(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting ResourceMetricsMonitor %q: %w", klog.KRef(namespace, name), err)
	}
	if errors.IsNotFound(err) {
		resource = &v1alpha1.ResourceMetricsMonitor{}
		resource.SetName(name)
	}

	return c.handleObject(ctx, resource, event)
}

func (c *Controller) handleObject(ctx context.Context, objectI interface{}, event string) error {
	logger := klog.FromContext(ctx)
	if objectI == nil {
		logger.Error(stderrors.New("received nil object for handling, skipping"), "error handling object")

		return nil
	}
	var object metav1.Object
	var ok bool
	if object, ok = objectI.(metav1.Object); !ok {
		tombstone, ok := objectI.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Error(stderrors.New("error decoding object, invalid type"), "error handling object")

			return nil
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			logger.Error(stderrors.New("error decoding object tombstone, invalid type"), "error handling object")

			return nil
		}
		logger.V(1).Info("Recovered", "key", klog.KObj(object))
	}
	logger = klog.LoggerWithValues(klog.FromContext(ctx), "key", klog.KObj(object), "event", event)
	logger.V(1).Info("Processing object")
	switch o := object.(type) {
	case *v1alpha1.ResourceMetricsMonitor:
		return c.handleEvent(ctx, &c.stores, event, o)
	default:
		logger.Error(stderrors.New("unknown object type"), "cannot handle object")

		return nil
	}
}
