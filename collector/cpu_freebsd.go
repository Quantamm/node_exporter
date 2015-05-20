// +build !nostat

package collector

import (
	"errors"
	"os"
	"strconv"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
)

/*
#cgo LDFLAGS: -lkvm
#include <fcntl.h>
#include <kvm.h>
#include <sys/param.h>
#include <sys/pcpu.h>
#include <sys/resource.h>
*/
import "C"

const ()

type statCollector struct {
	config Config
	cpu    *prometheus.CounterVec
}

func init() {
	Factories["cpu"] = NewStatCollector
}

// Takes a config struct and prometheus registry and returns a new Collector exposing
// network device stats.
func NewStatCollector(config Config) (Collector, error) {
	return &statCollector{
		config: config,
		cpu: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "cpu",
				Help:      "Seconds the cpus spent in each mode.",
			},
			[]string{"cpu", "mode"},
		),
	}, nil
}

// Expose cpu stats using kvm
func (c *statCollector) Update(ch chan<- prometheus.Metric) (err error) {
	if os.Geteuid() != 0 && os.Getegid() != 2 {
		return errors.New("Caller should be either root user or kmem group to access /dev/mem")
	}

	var errbuf *C.char
	kd := C.kvm_open(nil, nil, nil, C.O_RDONLY, errbuf)
	if errbuf != nil {
		return errors.New("Failed to call kvm_open().")
	}
	defer C.kvm_close(kd)

	ncpus := C.kvm_getncpus(kd)
	for i := 0; i < int(ncpus); i++ {
		pcpu := C.kvm_getpcpu(kd, C.int(i))
		cp_time := ((*C.struct_pcpu)(unsafe.Pointer(pcpu))).pc_cp_time
		c.cpu.With(prometheus.Labels{"cpu": strconv.Itoa(i), "mode": "user"}).Set(float64(cp_time[C.CP_USER]))
		c.cpu.With(prometheus.Labels{"cpu": strconv.Itoa(i), "mode": "nice"}).Set(float64(cp_time[C.CP_NICE]))
		c.cpu.With(prometheus.Labels{"cpu": strconv.Itoa(i), "mode": "system"}).Set(float64(cp_time[C.CP_SYS]))
		c.cpu.With(prometheus.Labels{"cpu": strconv.Itoa(i), "mode": "interrupt"}).Set(float64(cp_time[C.CP_INTR]))
		c.cpu.With(prometheus.Labels{"cpu": strconv.Itoa(i), "mode": "idle"}).Set(float64(cp_time[C.CP_IDLE]))
	}
	c.cpu.Collect(ch)
	return err
}
