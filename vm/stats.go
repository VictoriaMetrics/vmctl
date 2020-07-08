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
		s.samples, float64(s.samples)/s.importDuration.Seconds(),
		byteCountSI(int64(s.bytes)), byteCountSI(int64(float64(s.bytes)/s.importDuration.Seconds())),
		s.requests, s.retries)
}
