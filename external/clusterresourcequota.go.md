## Custom collector exammple

The following example is guaranteed to work with `f3c2a8deff2f612c4b26157d6cd1bdc008118604`.

```go
package external

import (
	"context"
	"strings"

	v1 "github.com/openshift/api/quota/v1"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"k8s.io/kube-state-metrics/v2/pkg/metric"
	generator "k8s.io/kube-state-metrics/v2/pkg/metric_generator"
	metricsstore "k8s.io/kube-state-metrics/v2/pkg/metrics_store"
)

// clusterResourceQuotaCollector implements the collectors interface.
var _ collectors = &clusterResourceQuotaCollector{}

type clusterResourceQuotaCollector struct {
}

func (c *clusterResourceQuotaCollector) Register() {
	collectorsInstance.Register(c)
}

func (c *clusterResourceQuotaCollector) GVKR() gvkr {
	return gvkr{
		GroupVersionKind:     schema.GroupVersionKind{Group: v1.GroupName, Version: v1.GroupVersion.String(), Kind: "ClusterResourceQuota"},
		GroupVersionResource: schema.GroupVersionResource{Group: v1.GroupName, Version: v1.GroupVersion.String(), Resource: "clusterresourcequotas"},
	}
}

func (c *clusterResourceQuotaCollector) BuildCollector(ctx context.Context, kubeconfig string) *metricsstore.MetricsStore {
	quotaMetricFamilies := []generator.FamilyGenerator{
		{
			Name: "openshift_clusterresourcequota_selector",
			Type: metric.Gauge,
			Help: "Selector of clusterresource quota, which defines the affected namespaces.",
			GenerateFunc: wrapClusterResourceQuotaFunc(func(r *v1.ClusterResourceQuota) *metric.Family {
				family := metric.Family{}

				sel := r.Spec.Selector
				labelKeys := []string{"type", "key", "value"}
				for key, value := range sel.AnnotationSelector {
					family.Metrics = append(family.Metrics, &metric.Metric{
						LabelKeys:   labelKeys,
						LabelValues: []string{"annotation", key, value},
						Value:       float64(1),
					})
				}

				if sel.LabelSelector != nil {
					labelKeys := []string{"type", "key", "value"}

					for key, value := range sel.LabelSelector.MatchLabels {
						family.Metrics = append(family.Metrics, &metric.Metric{
							LabelKeys:   labelKeys,
							LabelValues: []string{"match-labels", key, value},
							Value:       float64(1),
						})
					}

					labelKeys = []string{"type", "operator", "key", "values"}
					for _, labelReq := range sel.LabelSelector.MatchExpressions {
						family.Metrics = append(family.Metrics, &metric.Metric{
							LabelKeys:   labelKeys,
							LabelValues: []string{"match-expressions", string(labelReq.Operator), labelReq.Key, strings.Join(labelReq.Values, ",")},
							Value:       float64(1),
						})
					}
				}

				return &family
			}),
		},
	}

	store := metricsstore.NewMetricsStore(
		generator.ExtractMetricFamilyHeaders(quotaMetricFamilies),
		generator.ComposeMetricGenFuncs(quotaMetricFamilies),
	)

	for _, ns := range []string{metav1.NamespaceAll} {
		lw := createClusterResourceQuotaListWatch(ctx, kubeconfig, ns)
		reflector := cache.NewReflector(&lw, &v1.ClusterResourceQuota{}, store, 0)
		go reflector.Run(ctx.Done())
	}

	return store
}

func wrapClusterResourceQuotaFunc(f func(config *v1.ClusterResourceQuota) *metric.Family) func(interface{}) *metric.Family {
	return func(obj interface{}) *metric.Family {
		quota, ok := obj.(*v1.ClusterResourceQuota)
		if !ok {
			klog.Errorf("unexpected type %T when processing ClusterResourceQuota", obj)

			return &metric.Family{}
		}
		metricFamily := f(quota)

		descClusterResourceQuotaLabelsDefaultLabels := []string{"name"}
		for _, m := range metricFamily.Metrics {
			m.LabelKeys = append(descClusterResourceQuotaLabelsDefaultLabels, m.LabelKeys...)
			m.LabelValues = append([]string{quota.Name}, m.LabelValues...)
		}

		return metricFamily
	}
}

func createClusterResourceQuotaListWatch(ctx context.Context, kubeconfig, _ string) cache.ListWatch {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("cannot create quota config: %v", err)
	}
	client, err := quotaclient.NewForConfig(config)
	if err != nil {
		klog.Fatalf("cannot create quota client: %v", err)
	}

	return cache.ListWatch{
		ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
			return client.QuotaV1().ClusterResourceQuotas().List(ctx, opts)
		},
		WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
			return client.QuotaV1().ClusterResourceQuotas().Watch(ctx, opts)
		},
	}
}
```

