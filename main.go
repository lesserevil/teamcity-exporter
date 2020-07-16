package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
)

const (
	namespace     = "teamcity"
	exporterName  = "teamcity_queue_exporter"
	reasonDefault = "There are no compatible or available agents for this build"
)

var (
	showVersion = flag.Bool("version", false, "Prints version information and exit")

	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of TeamCity successful",
		nil, nil,
	)

	buildLabels = []string{"reason", "project", "buildId", "pool", "winOk", "linOk", "macOk"}

	agentLabels = []string{"pool", "os", "enabled", "authorized", "connected", "project", "busy"}

	buildQueueWaitCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_wait_count"),
		"How many builds in queue waiting in queue",
		buildLabels, nil,
	)

	agentInfoCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "agent_type_count"),
		"How many agents by metadata",
		agentLabels, nil,
	)
)

func init() {
	prometheus.MustRegister(version.NewCollector(exporterName))
}

func versionInfo() {
	fmt.Println(version.Print(exporterName))
	os.Exit(0)
}

func main() {
	flag.Parse()

	if strings.ToLower(os.Getenv("TE_DEBUG")) == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if *showVersion == true {
		versionInfo()
	}

	logrus.Infof("Starting %s %s...", exporterName, version.Version)

	config := NewConfig()
	if err := config.Load(); err != nil {
		logrus.Errorf("Configuration error: %v", err)
		os.Exit(1)
	}

	exporter := NewExporter(config)
	prometheus.MustRegister(exporter)

	http.Handle(config.metricPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>TeamCity Queue Exporter v` + version.Version + `</title></head>
			<body>
			<h1>TeamCity Queue Exporter v` + version.Version + `</h1>
			<p><a href='` + config.metricPath + `'>Metrics</a></p>
			</body>
			</html>
		`))
	})
	logrus.Fatal(http.ListenAndServe(config.listenAddress, nil))

}
