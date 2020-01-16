package vm

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Importer performs insertion of timeseries
// via VictoriaMetrics import protocol
// see https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master#how-to-import-time-series-data
type Importer struct {
	addr       string
	importPath string
	compress   bool

	close  chan struct{}
	input  chan *TimeSeries
	errors chan *ImportError

	wg   sync.WaitGroup
	once sync.Once

	s stats
}

// Config contains list of params to configure
// the Importer
type Config struct {
	// VictoriaMetrics address to perform import requests
	//   --httpListenAddr value for single node version
	//   --httpListenAddr value of VMSelect  component for cluster version
	Addr string
	// Concurrency defines number of worker
	// performing the import requests concurrently
	Concurrency uint8
	// Whether to apply gzip compression
	Compress bool
	// AccountID for cluster version
	// Less than 0 assumes single node version
	AccountID int
}

type stats struct {
	datapoints uint64
	bytes      uint64
	requests   uint64
	// todo: latency
}

func (im Importer) Stats() string {
	return im.s.String()
}

func (s stats) String() string {
	datapoints := atomic.LoadUint64(&s.datapoints)
	bytes := atomic.LoadUint64(&s.bytes)
	requests := atomic.LoadUint64(&s.requests)
	return fmt.Sprintf("  datapoints: %d;\n  bytes: %s;\n  import requests: %d",
		datapoints, byteCountSI(int64(bytes)), requests)
}

func NewImporter(cfg Config) (*Importer, error) {
	if cfg.Concurrency < 1 {
		return nil, fmt.Errorf("concurrency can't be lower than 1")
	}

	addr := strings.TrimRight(cfg.Addr, "/")

	// if single version
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master#how-to-import-time-series-data
	importPath := addr + "/api/v1/import"
	if cfg.AccountID != -1 {
		// if cluster version
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/tree/cluster#url-format
		importPath = fmt.Sprintf("%s/insert/%d/api/v1/import", addr, uint32(cfg.AccountID))
	}

	im := &Importer{
		addr:       addr,
		importPath: importPath,
		compress:   cfg.Compress,
		close:      make(chan struct{}),
		input:      make(chan *TimeSeries, cfg.Concurrency*4),
		errors:     make(chan *ImportError),
	}
	if err := im.Ping(); err != nil {
		return nil, fmt.Errorf("ping to %q failed: %s", addr, err)
	}

	im.wg.Add(int(cfg.Concurrency))
	for i := 0; i < int(cfg.Concurrency); i++ {
		go func() {
			defer im.wg.Done()
			im.startWorker()
		}()
	}

	return im, nil
}

// ImportError is type of error generated
// in case of unsuccessful import request
type ImportError struct {
	// The batch of timeseries that failed
	Batch []*TimeSeries
	// The error that appeared during insert
	Err error
}

// Errors returns a channel for receiving
// import errors if any
func (im *Importer) Errors() chan *ImportError { return im.errors }

// Input returns a channel for sending timeseries
// that need to be imported
func (im *Importer) Input() chan<- *TimeSeries { return im.input }

// Close sends signal to all goroutines to exit
// and waits until they are finished
func (im *Importer) Close() {
	im.once.Do(func() {
		close(im.close)
		im.wg.Wait()
	})
}

// TODO: make configurable
const batchDataPoints = 5e4

func (im *Importer) startWorker() {
	var batch []*TimeSeries
	var dataPoints int
	for {
		select {
		case <-im.close:
			if err := im.Import(batch); err != nil {
				im.errors <- &ImportError{
					Batch: batch,
					Err:   err,
				}
			}
			return
		case ts := <-im.input:
			batch = append(batch, ts)
			dataPoints += len(ts.Values)
			if dataPoints < batchDataPoints {
				continue
			}
			if err := im.flush(batch); err != nil {
				im.errors <- &ImportError{
					Batch: batch,
					Err:   err,
				}
				// make a new batch, since old one was referenced as err
				batch = make([]*TimeSeries, len(batch))
			}
			batch = batch[:0]
			dataPoints = 0
		}
	}
}

const (
	// TODO: make configurable
	backoffRetries     = 5
	backoffFactor      = 1.5
	backoffMinDuration = time.Second
)

func (im *Importer) flush(b []*TimeSeries) error {
	var err error
	for i := 0; i < backoffRetries; i++ {
		err = im.Import(b)
		if err == nil {
			return nil
		}
		backoff := float64(backoffMinDuration) * math.Pow(backoffFactor, float64(i))
		time.Sleep(time.Duration(backoff))
	}
	return fmt.Errorf("import failed with %d retries: %s", backoffRetries, err)
}

func (im *Importer) Ping() error {
	url := fmt.Sprintf("%s/health", im.addr)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("cannot create request to %q: %s", im.addr, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

func (im *Importer) Import(tsBatch []*TimeSeries) error {
	pr, pw := io.Pipe()
	req, err := http.NewRequest("POST", im.importPath, pr)
	if err != nil {
		return fmt.Errorf("cannot create request to %q: %s", im.addr, err)
	}
	if im.compress {
		req.Header.Set("Content-Encoding", "gzip")
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("unexpected error when performing request to %q: %s", im.addr, err)
			return
		}
		if resp.StatusCode != http.StatusNoContent {
			log.Printf("unexpected response code from %q: %d", im.addr, resp.StatusCode)
		}
	}()

	w := io.Writer(pw)
	if im.compress {
		zw, err := gzip.NewWriterLevel(pw, 1)
		if err != nil {
			return fmt.Errorf("unexpected error when creating gzip writer: %s", err)
		}
		w = zw
	}
	bw := bufio.NewWriterSize(w, 16*1024)

	var totalDP, totalBytes int
	for _, ts := range tsBatch {
		n, err := ts.write(bw)
		if err != nil {
			return fmt.Errorf("write err: %s", err)
		}
		totalBytes += n
		totalDP += len(ts.Values)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if im.compress {
		err := w.(*gzip.Writer).Close()
		if err != nil {
			return err
		}
	}
	if err := pw.Close(); err != nil {
		return err
	}
	wg.Wait()

	atomic.AddUint64(&im.s.bytes, uint64(totalBytes))
	atomic.AddUint64(&im.s.datapoints, uint64(totalDP))
	atomic.AddUint64(&im.s.requests, 1)
	return err
}

func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
