package vm

import (
	"fmt"
	"sync"
	"time"
)

type stats struct {
	sync.Mutex
	samples        uint64
	bytes          uint64
	requests       uint64
	retries        uint64
	startTime      time.Time
	importDuration time.Duration
	idleDuration   time.Duration
}

func (s *stats) String() string {
	s.Lock()
	defer s.Unlock()

	importDurationS := s.importDuration.Seconds()
	var samplesPerS float64
	if s.samples > 0 && importDurationS > 0 {
		samplesPerS = float64(s.samples) / importDurationS
	}
	bytesPerS := byteCountSI(0)
	if s.bytes > 0 && importDurationS > 0 {
		bytesPerS = byteCountSI(int64(float64(s.bytes) / importDurationS))
	}

	return fmt.Sprintf("VictoriaMetrics importer stats:\n"+
		"  time spent while waiting: %v;\n"+
		"  time spent while importing: %v;\n"+
		"  total samples: %d;\n"+
		"  samples/s: %.2f;\n"+
		"  total bytes: %s;\n"+
		"  bytes/s: %s;\n"+
		"  import requests: %d;\n"+
		"  import requests retries: %d;",
		s.idleDuration, s.importDuration,
		s.samples, samplesPerS,
		byteCountSI(int64(s.bytes)), bytesPerS,
		s.requests, s.retries)
}
