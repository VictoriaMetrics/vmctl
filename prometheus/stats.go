package prometheus

import (
	"fmt"
	"time"
)

type Stats struct {
	MinTime       int64
	MaxTime       int64
	Samples       uint64
	Series        uint64
	Blocks        int
	SkippedBlocks int
}

func (s Stats) String() string {
	return fmt.Sprintf("Prometheus snapshot stats:\n"+
		"  blocks found: %d;\n"+
		"  blocks skipped: %d;\n"+
		"  min time: %d (%v);\n"+
		"  max time: %d (%v);\n"+
		"  samples: %d;\n"+
		"  series: %d.\n"+
		"Filter is not taken into account for series and samples numbers.",
		s.Blocks, s.SkippedBlocks,
		s.MinTime, time.Unix(s.MinTime/1e3, 0).Format(time.RFC3339),
		s.MaxTime, time.Unix(s.MaxTime/1e3, 0).Format(time.RFC3339),
		s.Samples, s.Series)
}
