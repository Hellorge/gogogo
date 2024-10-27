package profiler

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	_ "net/http/pprof" // This imports the pprof HTTP handlers
)

type Profiler struct {
	CPUProfile *os.File
	MemProfile *os.File
	StartTime  time.Time
}

func New() *Profiler {
	return &Profiler{}
}

func (p *Profiler) Start(cpuProfilePath, memProfilePath string) error {
	var err error
	p.CPUProfile, err = os.Create(cpuProfilePath)
	if err != nil {
		return fmt.Errorf("could not create CPU profile: %v", err)
	}
	if err := pprof.StartCPUProfile(p.CPUProfile); err != nil {
		p.CPUProfile.Close()
		return fmt.Errorf("could not start CPU profile: %v", err)
	}

	p.MemProfile, err = os.Create(memProfilePath)
	if err != nil {
		pprof.StopCPUProfile()
		p.CPUProfile.Close()
		return fmt.Errorf("could not create memory profile: %v", err)
	}

	p.StartTime = time.Now()
	return nil
}

func (p *Profiler) Stop() error {
	pprof.StopCPUProfile()
	p.CPUProfile.Close()

	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(p.MemProfile); err != nil {
		return fmt.Errorf("could not write memory profile: %v", err)
	}
	p.MemProfile.Close()

	return nil
}

func (p *Profiler) GetStats() ProfileStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return ProfileStats{
		Uptime:       time.Since(p.StartTime),
		AllocatedMem: m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		Sys:          m.Sys,
		NumGC:        m.NumGC,
	}
}

type ProfileStats struct {
	Uptime       time.Duration
	AllocatedMem uint64
	TotalAlloc   uint64
	Sys          uint64
	NumGC        uint32
}

// StartHTTPProfile starts the pprof HTTP server for on-demand profiling
func StartHTTPProfile(addr string) error {
	return http.ListenAndServe(addr, nil)
}
