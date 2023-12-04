//go:build libpfm && cgo
// +build libpfm,cgo

package rawcollector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/Rouzip/goperf/pkg/utils"
	"golang.org/x/sys/unix"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// #cgo CFLAGS: -I/usr/include
// #cgo LDFLAGS: -lpfm
// #include <perfmon/pfmlib.h>
// #include <stdlib.h>
// #include <string.h>
import "C"

var (
	initLibpfm     sync.Once
	finalizeLibpfm sync.Once
	bufPool        sync.Pool
)

func init() {
	initLibpfm.Do(func() {
		if err := C.pfm_initialize(); err != C.PFM_SUCCESS {
			panic("failed to initialize libpfm")
		}
	})
	bufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 24+16*len(utils.EventsToCollect))
		},
	}
}

func Finalize() {
	finalizeLibpfm.Do(func() {
		C.pfm_terminate()
	})
}

type EventsGroup struct {
	EventsGroup []Group
}

type Group struct {
	Events []string
}

type RawCollector struct {
	CGroupFd     *os.File
	CPUCollector map[int]group // groups of events
	// assume that all events are unique
	Values     map[string]float64
	EventIDMap map[uint64]string
	Pod        *v1.Pod
	Container  *v1.ContainerStatus
	valCh      chan perfValue
	idCh       chan perfId
}

type perfValue struct {
	Value uint64
	ID    uint64
}

type perfId struct {
	id    uint64
	event string
}

type group struct {
	leaderName string
	eventNames []string
	fds        map[int]io.ReadCloser
}

func (g *group) createEnabledFds(cgroupFd *os.File, idMap chan perfId) error {
	eventConfigMap := make(map[string]*unix.PerfEventAttr)
	fdMap := make(map[int]int)
	for _, event := range g.eventNames {
		config, err := createPerfConfig(event)
		config.Sample_type = unix.PERF_SAMPLE_IDENTIFIER
		config.Read_format = unix.PERF_FORMAT_GROUP | unix.PERF_FORMAT_TOTAL_TIME_ENABLED | unix.PERF_FORMAT_TOTAL_TIME_RUNNING | unix.PERF_FORMAT_ID
		config.Size = uint32(unsafe.Sizeof(unix.PerfEventAttr{}))
		// TODO: change location of free?
		defer C.free(unsafe.Pointer(config))
		if err != nil {
			return err
		}
		eventConfigMap[event] = config
	}

	for i := 0; i < utils.CPUNUM; i++ {
		var leaderFd int
		var err error
		for _, event := range g.eventNames {
			if event == g.leaderName {
				attr := eventConfigMap[event]
				attr.Bits = unix.PerfBitDisabled | unix.PerfBitInherit
				leaderFd, err = unix.PerfEventOpen(attr, int(cgroupFd.Fd()), i, -1, unix.PERF_FLAG_PID_CGROUP|unix.PERF_FLAG_FD_CLOEXEC)
				if err != nil {
					return err
				}
				fdMap[i] = leaderFd
				perfFd := os.NewFile(uintptr(leaderFd), g.leaderName)
				if perfFd == nil {
					return fmt.Errorf("failed to create perfFd")
				}
				g.fds[i] = perfFd
				var id uint64
				_, _, err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(leaderFd), unix.PERF_EVENT_IOC_ID, uintptr(unsafe.Pointer(&id)))
				if err != 0 {
					return err
				}
				idMap <- perfId{
					id:    id,
					event: event,
				}
			} else {
				go func(event string, cpu int) {
					attr := eventConfigMap[event]
					attr.Bits = unix.PerfBitInherit
					fd, err := unix.PerfEventOpen(attr, int(cgroupFd.Fd()), cpu, leaderFd, unix.PERF_FLAG_PID_CGROUP|unix.PERF_FLAG_FD_CLOEXEC)
					if err != nil {
						klog.Error(err)
					}
					var id uint64
					_, _, err = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), unix.PERF_EVENT_IOC_ID, uintptr(unsafe.Pointer(&id)))
					if err != syscall.Errno(0) {
						klog.Error(err)
					}
					idMap <- perfId{
						id:    id,
						event: event,
					}
				}(event, i)
			}
		}
	}
	for _, fd := range fdMap {
		if err := unix.IoctlSetInt(fd, unix.PERF_EVENT_IOC_RESET, 1); err != nil {
			return err
		}
		if err := unix.IoctlSetInt(fd, unix.PERF_EVENT_IOC_ENABLE, 1); err != nil {
			return err
		}
	}
	return nil
}

func (g *group) collect(ch chan perfValue) error {
	var wg sync.WaitGroup
	wg.Add(len(g.fds))
	for _, fd := range g.fds {
		go func(fd io.ReadCloser) {
			defer wg.Done()
			buf := bufPool.Get().([]byte)
			defer bufPool.Put(buf)
			_, err := fd.Read(buf)
			if err != nil {
				klog.Error(err)
				return
			}
			type groupReadFormat struct {
				Nr          uint64
				TimeEnabled uint64
				TimeRunning uint64
			}

			header := &groupReadFormat{}
			reader := bytes.NewReader(buf)
			err = binary.Read(reader, binary.LittleEndian, header)
			if err != nil {
				klog.Error(err)
				return
			}
			scalingRatio := 1.0
			if header.TimeRunning != 0 && header.TimeEnabled != 0 {
				scalingRatio = float64(header.TimeRunning) / float64(header.TimeEnabled)
			}
			if scalingRatio != 0 {
				for i := 0; i < int(header.Nr); i++ {
					value := &perfValue{}
					err = binary.Read(reader, binary.LittleEndian, value)
					if err != nil {
						klog.Error(err)
						return
					}
					value.Value = uint64(float64(value.Value) / scalingRatio)
					ch <- *value
				}
			}
		}(fd)
	}
	wg.Wait()
	return nil
}

// just collect Cycles and Instructions for now one group? TODO: just collect core events
func NewRawCollector(pod *v1.Pod, container *v1.ContainerStatus, events EventsGroup) *RawCollector {
	rc := &RawCollector{
		CPUCollector: make(map[int]group),
	}
	fd, err := utils.CGroupFd(pod, container)
	if err != nil {
		klog.Fatal(err)
	}
	rc.CGroupFd = fd
	rc.Values = make(map[string]float64)
	rc.EventIDMap = make(map[uint64]string)
	rc.idCh = make(chan perfId)
	rc.Container = container
	rc.Pod = pod
	go func() {
		// FIXME: how to close this channel?
		// ctx?
		for idMap := range rc.idCh {
			rc.EventIDMap[idMap.id] = idMap.event
		}
	}()
	rc.valCh = make(chan perfValue)
	go func() {
		// FIXME: how to close this channel?
		for val := range rc.valCh {
			rc.Values[rc.EventIDMap[val.ID]] += float64(val.Value)
		}
	}()

	// for poc, change events to group
	perfGroup := &group{
		leaderName: "instructions",
		eventNames: utils.EventsToCollect,
		fds:        make(map[int]io.ReadCloser),
	}
	err = perfGroup.createEnabledFds(rc.CGroupFd, rc.idCh)
	if err != nil {
		klog.Fatal(err)
	}
	rc.CPUCollector[0] = *perfGroup

	// for poc, create a group for test, leader: instructions, instructions, cycles
	return rc
}

func (r *RawCollector) Collect() error {
	var wg sync.WaitGroup
	wg.Add(len(r.CPUCollector))
	for _, gc := range r.CPUCollector {
		// TODO: error fix
		go func(gc group) {
			defer wg.Done()
			gc.collect(r.valCh)
		}(gc)
	}
	wg.Wait()
	for _, gc := range r.CPUCollector {
		for _, fd := range gc.fds {
			fd.Close()
		}
	}
	return nil
}

func (r *RawCollector) Close() error {
	return r.CGroupFd.Close()
}

// caller must free the memory
func createPerfConfig(event string) (*unix.PerfEventAttr, error) {
	// https://pkg.go.dev/cmd/cgo OOM instread of check malloc error
	perfEventAttrPtr := C.malloc(C.ulong(unsafe.Sizeof(unix.PerfEventAttr{})))
	C.memset(perfEventAttrPtr, 0, C.ulong(unsafe.Sizeof(unix.PerfEventAttr{})))
	if err := pfmGetOsEventEncoding(event, perfEventAttrPtr); err != nil {
		return nil, err
	}

	return (*unix.PerfEventAttr)(perfEventAttrPtr), nil
}

// https://man7.org/linux/man-pages/man3/pfm_get_os_event_encoding.3.html
func pfmGetOsEventEncoding(event string, perfEventAttrPtr unsafe.Pointer) error {
	arg := pfmPerfEncodeArgT{}
	arg.attr = perfEventAttrPtr
	fstr := C.CString("")
	defer C.free(unsafe.Pointer(fstr))
	arg.size = C.ulong(unsafe.Sizeof(arg))
	eventCStr := C.CString(event)
	defer C.free(unsafe.Pointer(eventCStr))
	if err := C.pfm_get_os_event_encoding(eventCStr, C.PFM_PLM3, C.PFM_OS_PERF_EVENT, unsafe.Pointer(&arg)); err != C.PFM_SUCCESS {
		return fmt.Errorf("failed to get event encoding: %d", err)
	}
	return nil
}
