package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
)

const (
	namespace = "teamcity"
)

var (
	showVersion = flag.Bool("version", false, "Prints version information and exit")

	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of TeamCity successful.",
		nil, nil,
	)

	buildQueueCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_size"),
		"How many builds in queue at the last query",
		nil, nil,
	)
)

func init() {
	prometheus.MustRegister(version.NewCollector("teamcity_exporter"))
}

func versionInfo() {
	fmt.Println(version.Print("teamcity_exporter"))
	os.Exit(0)
}

func main() {
	flag.Parse()

	if *showVersion == true {
		versionInfo()
	}

	logrus.Info("Starting teamcity_exporter...")

	config := NewConfig()
	if err := config.Load(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	exporter := NewExporter(config)

	if err := prometheus.Register(exporter); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	exporter.requestEndpoint("QUE", nil)

	http.Handle(config.metricPath, promhttp.Handler())
	logrus.Fatal(http.ListenAndServe(config.listenAddress, nil))

}
