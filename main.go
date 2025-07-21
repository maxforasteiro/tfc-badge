package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var flagAddr = flag.String("addr", ":3030", "specify the address and port to listen on")
var metricsAddr = flag.String("metrics", ":9080", "specify the address and port to listen on for metrics")
var debug = flag.Bool("debug", false, "enable debug log output")
var persistence = flag.String("file", "", "path to file to store persistent cache in")

func main() {
	flag.Parse()

	store := NewCache()
	if *persistence != "" {
		err := func() error {
			cacheFile, err := os.Open(*persistence)
			if err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("error opening file: %v", err)
				}
				p := path.Dir(*persistence)
				if err := os.MkdirAll(p, 0666); err != nil {
					return fmt.Errorf("error creating path %s: %v", p, err)
				}
				f, err := os.Create(*persistence)
				if err != nil {
					return fmt.Errorf("error creating file %s: %v", *persistence, err)
				}
				cacheFile = f
			}
			if err := store.Restore(cacheFile); err != nil {
				return fmt.Errorf("error restoring cache from file %s: %v", *persistence, err)
			}
			if len(store.List()) > 0 {
				log.Printf("restored cache %d entries", len(store.List()))
			}
			return cacheFile.Close()
		}()
		if err != nil {
			log.Printf("error initialising persistent cache: %v", err)
		}
	}

	hooks := NewHookRunner()

	grafanaHost, hostOK := os.LookupEnv("GRAFANA_HOST")
	grafanaKey, keyOK := os.LookupEnv("GRAFANA_API_KEY")
	if hostOK && keyOK {
		log.Println("adding Grafana Annotation Hook")
		hooks.Fn["Grafana Annotation"] = GrafanaAnnotation(grafanaHost, grafanaKey)
	}

	a := AppServer{
		Store: store,
		Debug: *debug,
		Addr:  *flagAddr,
		Hook:  hooks.Hook(),
	}

	mc := &MetricsCollector{
		Store: store,
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(mc)

	// Application Server
	go func() {
		log.Printf("starting server on address %s", *flagAddr)
		if err := a.Run(); err != nil {
			log.Printf("error running http server: %v", err)
		}
	}()

	// Metrics Server
	go func() {
		log.Printf("starting server on address %s", *metricsAddr)
		http.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(*metricsAddr, promhttp.HandlerFor(reg, promhttp.HandlerOpts{})); err != nil {
			log.Printf("error running metrics server: %v", err)
		}
	}()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	s := <-done
	log.Printf("received signal: %v", s)
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		log.Fatalf("error shutting down server: %v", err)
	}
	log.Println("done shutting down server")

	if *persistence == "" {
		os.Exit(0)
	}

	cacheFile, err := os.OpenFile(*persistence, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Printf("error opening cache file %s: %v", *persistence, err)
	}
	if err := store.Dump(cacheFile); err != nil {
		log.Printf("error restoring cache from file %s: %v", *persistence, err)
	}
	if err := cacheFile.Close(); err != nil {
		log.Printf("error closing cache file: %v", err)
	}
	log.Printf("done dumping cache")
}
