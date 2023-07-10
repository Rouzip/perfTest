package gorawcollector

import (
	"os"

	"github.com/Rouzip/goperf/pkg/utils"
	"github.com/hodgesds/perf-utils"
	"golang.org/x/sys/unix"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// collector for container
type GoRawCollector struct {
	CGroupFd     *os.File
	CPUCollector map[int]perf.HardwareProfiler
	Cycle        uint64
	Instruction  uint64
	Pod          *v1.Pod
	Container    *v1.ContainerStatus
}

func NewGoRawCollector(pod *v1.Pod, container *v1.ContainerStatus) *GoRawCollector {
	grc := &GoRawCollector{
		CPUCollector: make(map[int]perf.HardwareProfiler),
		Pod:          pod,
		Container:    container,
	}
	fd, err := utils.CGroupFd(pod, container)
	if err != nil {
		klog.Fatal(err)
	}
	grc.CGroupFd = fd

	for i := 0; i < utils.CPUNUM; i++ {
		hp, err := perf.NewHardwareProfiler(int(grc.CGroupFd.Fd()), i, perf.RefCpuCyclesProfiler|perf.CpuInstrProfiler, unix.PERF_FLAG_PID_CGROUP)
		if err != nil {
			klog.Fatal(err)
		}
		grc.CPUCollector[i] = hp
	}

	for i := 0; i < utils.CPUNUM; i++ {
		if newErr := grc.CPUCollector[i].Start(); newErr != nil {
			klog.Fatal(newErr)
		}
	}

	return grc
}

// concurrency
func (r *GoRawCollector) Collect() error {
	for _, collector := range r.CPUCollector {
		profile := &perf.HardwareProfile{}
		collector.Profile(profile)
		r.Cycle += *profile.RefCPUCycles
		r.Instruction += *profile.Instructions
	}

	defer func() {
		for _, collecotr := range r.CPUCollector {
			collecotr.Stop()
			collecotr.Close()
		}
	}()
	return nil
}

func (r *GoRawCollector) Close() error {
	return r.CGroupFd.Close()
}
