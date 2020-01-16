package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "net/http/pprof"

	"github.com/cheggaaa/pb/v3"
	"github.com/urfave/cli/v2"
	"github.com/victoriametrics/vmctl/influx"
	"github.com/victoriametrics/vmctl/vm"
)

const version = "0.0.1"

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	app := &cli.App{
		Name:    "VictoriaMetrics migrate cli tool",
		Version: version,
		Commands: []*cli.Command{
			{
				Name:  "influx",
				Usage: "Migrate timeseries from InfluxDB",
				Flags: append(influxFlags, vmFlags...),
				Action: func(c *cli.Context) error {
					fmt.Println("InfluxDB import mode")

					iCfg := influx.Config{
						Addr:      c.String(influxAddr),
						Username:  c.String(influxUser),
						Password:  c.String(influxPassword),
						Database:  c.String(influxDB),
						Retention: c.String(influxRetention),
						Filter:    c.String(influxFilter),
						ChunkSize: c.Int(influxChunkSize),
					}
					influxClient, err := influx.NewClient(iCfg)
					if err != nil {
						return fmt.Errorf("failed to create influx client: %s", err)
					}

					vmCfg := vm.Config{
						Addr:        c.String(vmAddr),
						Concurrency: uint8(c.Int(vmConcurrency)),
						Compress:    c.Bool(vmCompress),
						AccountID:   c.Int(vmAccountID),
					}
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					return importInflux(influxClient, importer)
				},
			},
			{
				Name:  "prometheus",
				Usage: "Migrate timeseries from Prometheus [WIP]",
				Flags: vmFlags,
				Action: func(c *cli.Context) error {
					fmt.Println("Prometheus migrate action is not implemented yet")
					return nil
				},
			},
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		os.Exit(0)
	}()

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func importInflux(ic *influx.Client, importer *vm.Importer) error {
	series, err := ic.Explore()
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
	go func() {
		for {
			ie := <-importer.Errors()
			var errTS string
			for _, ts := range ie.Batch {
				errTS += fmt.Sprintf("%s for timestamps range %d - %d\n",
					ts.String(), ts.Timestamps[0], ts.Timestamps[len(ts.Timestamps)-1])
			}
			log.Fatalf("Import process failed for \n%sWith error: %s", errTS, ie.Err)
		}
	}()

	var total int
	for _, s := range series {
		cr, err := ic.FetchDataPoints(s)
		if err != nil {
			return fmt.Errorf("failed to fetch datapoints: %s", err)
		}
		name := fmt.Sprintf("%s_%s", s.Measurement, s.Field)
		labels := make([]vm.LabelPair, len(s.LabelPairs))
		for i, lp := range s.LabelPairs {
			labels[i] = vm.LabelPair{
				Name:  lp.Name,
				Value: lp.Value,
			}
		}

	chunks:
		for {
			time, values, err := cr.Next()
			if err == io.EOF {
				break chunks
			}
			total += len(values)
			importer.Input() <- &vm.TimeSeries{
				Name:       name,
				LabelPairs: labels,
				Timestamps: time,
				Values:     values,
			}
		}
		bar.Increment()
	}

	importer.Close()
	bar.Finish()
	log.Printf("Import finished:\n%s", importer.Stats())
	return nil
}
