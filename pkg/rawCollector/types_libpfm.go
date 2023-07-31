//go:build cgo
// +build cgo

package rawcollector

import (
	"C"
	"unsafe"
)

// GroupReadFormat allows to read perf event's values for grouped events.
// See https://man7.org/linux/man-pages/man2/perf_event_open.2.html section "Reading results" with PERF_FORMAT_GROUP specified.
type GroupReadFormat struct {
	Nr          uint64 /* The number of events */
	TimeEnabled uint64 /* if PERF_FORMAT_TOTAL_TIME_ENABLED */
	TimeRunning uint64 /* if PERF_FORMAT_TOTAL_TIME_RUNNING */
}

type Values struct {
	Value uint64 /* The value of the event */
	ID    uint64 /* if PERF_FORMAT_ID */
}

// pfmPerfEncodeArgT represents structure that is used to parse perf event nam
// into perf_event_attr using libpfm.
type pfmPerfEncodeArgT struct {
	attr unsafe.Pointer
	fstr unsafe.Pointer
	size C.size_t
	_    C.int // idx
	_    C.int // cpu
	_    C.int // flags
}
