package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"
)

const (
	namespace                 = "teamcity"
	exporterName              = "teamcity_exporter"
	reasonNoAgents            = "There are no compatible agents which can run this build"
	reasonDependencies        = "Build dependencies have not been built yet"
	reasonMaxConcurrentBuilds = "The maximum number of running builds for this configuration is reached"
	reasonResourceUnavailalbe = "Build is waiting for the following resource to become available"
)

var (
	showVersion = flag.Bool("version", false, "Prints version information and exit")

	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of TeamCity successful",
		nil, nil,
	)

	buildQueueCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_count"),
		"How many builds in queue at the last query",
		nil, nil,
	)

	buildQueueWaitOnAgentCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_wait_agent_count"),
		"How many builds in queue waiting on an availalble agent at last query",
		nil, nil,
	)

	buildQueueWaitOnConcurrentBuildCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_wait_concurrent_build_count"),
		"How many builds in queue not starting because of concurrent build limits at last query",
		nil, nil,
	)

	buildQueueWaitOnDependenciesCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_wait_dependencies_count"),
		"How many builds in queue waiting on dependent builds at last query",
		nil, nil,
	)

	buildQueueWaitOnSharedResourceCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "build_queue_wait_shared_resource_count"),
		"How many builds in queue waiting on availalbe shared resource at last query",
		nil, nil,
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
			<head><title>TeamCity Exporter v` + version.Version + `</title></head>
			<body>
			<h1>TeamCity Exporter v` + version.Version + `</h1>
			<p><a href='` + config.metricPath + `'>Metrics</a></p>
			</body>
			</html>
		`))
	})
	logrus.Fatal(http.ListenAndServe(config.listenAddress, nil))

}
