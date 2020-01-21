package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/victoriametrics/vmctl/influx"
	"github.com/victoriametrics/vmctl/vm"
)

const version = "0.0.1"

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	start := time.Now()
	app := &cli.App{
		Name:    "vmctl",
		Usage:   "Victoria metrics command-line tool",
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
						User:        c.String(vmUser),
						Password:    c.String(vmPassword),
						Concurrency: uint8(c.Int(vmConcurrency)),
						Compress:    c.Bool(vmCompress),
						AccountID:   c.Int(vmAccountID),
					}
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					processor := newInfluxProcessor(influxClient, importer, c.Int(influxConcurrency))
					return processor.run()
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
		fmt.Println("\r- Execution cancelled")
		os.Exit(0)
	}()

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Total time: %v\n", time.Since(start))
}
