package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Exporter struct {
	config     *Config
	httpClient *http.Client
}

type TeamCityServer struct {
	Version   string `json:"version"`
	StartTime string `json:"startTime"`
}

type TeamCityBuildSummary struct {
	ID int `json:"id"`
}

type TeamCityBuildQueue struct {
	Count  int                    `json:"count"`
	Href   string                 `json:"href"`
	Builds []TeamCityBuildSummary `json:"build"`
}

type TeamCityBuild struct {
	ID         int    `json:"id"`
	WaitReason string `json:"waitReason"`
}

func NewExporter(config *Config) *Exporter {
	return &Exporter{
		config: config,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (e *Exporter) requestEndpoint(route string, v interface{}) error {
	u := *e.config.apiEndpointUrl
	u.Path = route
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(e.config.apiLogin, e.config.apiPassword)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error requesting url: %s (%s)", req.URL.String(), resp.Status)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&v); err != nil {
		return err
	}
	return nil
}

func (e *Exporter) GetTeamCityServerInformation() (*TeamCityServer, error) {
	var teamCity *TeamCityServer
	err := e.requestEndpoint("app/rest/server", &teamCity)
	if err != nil {
		return nil, err
	}
	return teamCity, nil
}

func (e *Exporter) GetTeamCityBuildQueue() (*TeamCityBuildQueue, error) {
	var teamCityBuildQueue *TeamCityBuildQueue
	err := e.requestEndpoint("app/rest/buildQueue", &teamCityBuildQueue)
	if err != nil {
		return nil, err
	}
	return teamCityBuildQueue, nil
}

func (e *Exporter) GetTeamCityQueuedBuild(id int) (*TeamCityBuild, error) {
	var teamCityBuild *TeamCityBuild
	err := e.requestEndpoint(fmt.Sprintf("app/rest/buildQueue/id:%d", id), &teamCityBuild)
	if err != nil {
		return nil, err
	}
	return teamCityBuild, nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	_, err := e.GetTeamCityServerInformation()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0.0,
		)
		ch <- prometheus.MustNewConstMetric(
			buildQueueCount, prometheus.GaugeValue, 0.0,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 1.0,
		)
		bq, err := e.GetTeamCityBuildQueue()
		if err != nil {
			ch <- prometheus.MustNewConstMetric(
				buildQueueCount, prometheus.GaugeValue, 0.0,
			)
		} else {
			ch <- prometheus.MustNewConstMetric(
				buildQueueCount, prometheus.GaugeValue, float64(bq.Count),
			)
			var reasons map[string]int
			reasons = make(map[string]int)
			for i := 0; i < len(bq.Builds); i++ {
				b, err := e.GetTeamCityQueuedBuild(bq.Builds[i].ID)
				if err != nil {
				} else {
					reason := b.WaitReason
					reasons[strings.FieldsFunc(reason, func(c rune) bool { return c == 58 })[0]]++
				}
			}
			ch <- prometheus.MustNewConstMetric(
				buildQueueWaitOnAgentCount, prometheus.GaugeValue, float64(reasons[reasonNoAgents]),
			)
			ch <- prometheus.MustNewConstMetric(
				buildQueueWaitOnConcurrentBuildCount, prometheus.GaugeValue, float64(reasons[reasonMaxConcurrentBuilds]),
			)
			ch <- prometheus.MustNewConstMetric(
				buildQueueWaitOnDependenciesCount, prometheus.GaugeValue, float64(reasons[reasonDependencies]),
			)
			ch <- prometheus.MustNewConstMetric(
				buildQueueWaitOnSharedResourceCount, prometheus.GaugeValue, float64(reasons[reasonResourceUnavailalbe]),
			)
		}
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- buildQueueCount
}
