package external

import (
	"io"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-state-metrics/pkg/collector"
)

// collectors defines behaviors to implement custom Go-based collectors for the "main" instance.
type gvkr struct {
	schema.GroupVersionKind
	schema.GroupVersionResource
}
type collectors interface {
	BuildCollector(kubeconfig string) *collector.Collector
	GVKR() gvkr
	Register()
}

type collectorsType struct {
	kubeconfig      string
	collectors      []collectors
	builtCollectors []*collector.Collector
}

func (ct *collectorsType) SetKubeConfig(kubeconfig string) *collectorsType {
	ct.kubeconfig = kubeconfig

	return ct
}

func (ct *collectorsType) Register(c collectors) {
	ct.collectors = append(ct.collectors, c)
	ct.builtCollectors = append(ct.builtCollectors, c.BuildCollector(ct.kubeconfig))
}

func (ct *collectorsType) Build() {
	for _, c := range ct.collectors {
		c.Register()
	}
}

func (ct *collectorsType) Write(w io.Writer) {
	for _, c := range ct.builtCollectors {
		c.Collect(w)
	}
}

var collectorsInstance = &collectorsType{
	collectors: []collectors{
		// Add collectors below:
		&clusterResourceQuotaCollector{},
	},
}

//nolint:revive
func CollectorsGetter() *collectorsType {
	return collectorsInstance
}
