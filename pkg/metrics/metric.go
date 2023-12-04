package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
)

const (
	Cycles       = "cylces"
	Instructions = "instructions"
	Namespace    = "namespace"
	Pod          = "pod"
	Container    = "container"
	ContainerID  = "containerid"
	CPIFieid     = "cpi_type"
	CacheMiss    = "LONGEST_LAT_CACHE.MISS"
)

var (
	ContainerCPI = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "container_cpi",
	}, []string{Namespace, Pod, Container, ContainerID, CPIFieid})
	CPICollectors = []prometheus.Collector{ContainerCPI}
)

func init() {
	prometheus.MustRegister(CPICollectors...)
}

func RecordCPI(container *v1.ContainerStatus, pod *v1.Pod, cycles, ins float64, cachemiss float64) {
	labels := prometheus.Labels{}
	labels[Namespace] = pod.Namespace
	labels[Pod] = pod.Name
	labels[Container] = container.Name
	labels[ContainerID] = container.ContainerID
	labels[CPIFieid] = Cycles
	ContainerCPI.With(labels).Set(cycles)

	labels[CPIFieid] = Instructions
	ContainerCPI.With(labels).Set(ins)

	labels[CPIFieid] = CacheMiss
	ContainerCPI.With(labels).Set(cachemiss)
}
