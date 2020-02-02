package main

import (
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/cheggaaa/pb/v3"
	"github.com/victoriametrics/vmctl/influx"
	"github.com/victoriametrics/vmctl/vm"
)

type influxProcessor struct {
	ic        *influx.Client
	im        *vm.Importer
	cc        int
	separator string
}

func newInfluxProcessor(ic *influx.Client, im *vm.Importer, cc int, separator string) *influxProcessor {
	if cc < 1 {
		cc = 1
	}
	return &influxProcessor{
		ic:        ic,
		im:        im,
		cc:        cc,
		separator: separator,
	}
}

func (ip *influxProcessor) run() error {
	series, err := ip.ic.Explore()
	if err != nil {
		return fmt.Errorf("explore query failed: %s", err)
	}
	if len(series) < 1 {
		return fmt.Errorf("found no timeseries to export")
	}

	question := fmt.Sprintf("Found %d timeseries to import. Continue?", len(series))
	if !prompt(question) {
		return nil
	}

	bar := pb.StartNew(len(series))
	seriesCh := make(chan *influx.Series)
	errCh := make(chan error)

	var wg sync.WaitGroup
	wg.Add(ip.cc)
	for i := 0; i < ip.cc; i++ {
		go func() {
			defer wg.Done()
			for s := range seriesCh {
				if err := ip.do(s); err != nil {
					errCh <- fmt.Errorf("request failed for %q.%q: %s", s.Measurement, s.Field, err)
					return
				}
				bar.Increment()
			}
		}()
	}

	// any error breaks the import
	for _, s := range series {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("influx error: %s", infErr)
		case vmErr := <-ip.im.Errors():
			var errTS string
			for _, ts := range vmErr.Batch {
				errTS += fmt.Sprintf("%s for timestamps range %d - %d\n",
					ts.String(), ts.Timestamps[0], ts.Timestamps[len(ts.Timestamps)-1])
			}
			return fmt.Errorf("Import process failed for: \n%swith error: %s", errTS, vmErr.Err)
		case seriesCh <- s:
		}
	}

	close(seriesCh)
	wg.Wait()
	ip.im.Close()
	bar.Finish()
	log.Println("Import finished!")
	log.Print(ip.im.Stats())
	return nil
}

const dbLabel = "db"

func (ip *influxProcessor) do(s *influx.Series) error {
	cr, err := ip.ic.FetchDataPoints(s)
	if err != nil {
		return fmt.Errorf("failed to fetch datapoints: %s", err)
	}
	defer cr.Close()
	var name string
	if s.Measurement != "" {
		name = fmt.Sprintf("%s%s%s", s.Measurement, ip.separator, s.Field)
	} else {
		name = s.Field
	}

	labels := make([]vm.LabelPair, len(s.LabelPairs))
	var containsDBLabel bool
	for i, lp := range s.LabelPairs {
		if lp.Name == dbLabel {
			containsDBLabel = true
			break
		}
		labels[i] = vm.LabelPair{
			Name:  lp.Name,
			Value: lp.Value,
		}
	}
	if !containsDBLabel {
		labels = append(labels, vm.LabelPair{
			Name:  dbLabel,
			Value: ip.ic.Database(),
		})
	}

	for {
		time, values, err := cr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		// skip empty results
		if len(time) < 1 {
			continue
		}
		ip.im.Input() <- &vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: time,
			Values:     values,
		}
	}
}
