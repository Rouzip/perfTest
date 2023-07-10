package rawcollector

import v1 "k8s.io/api/core/v1"

type RawCollector struct {
}

func NewRawCollector(pod *v1.Pod, container *v1.ContainerStatus) *RawCollector {
	return &RawCollector{}
}

func (r *RawCollector) Collect() error {
	return nil
}

func (r *RawCollector) Close() error {
	return nil
}
