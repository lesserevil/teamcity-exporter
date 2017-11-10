package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type TeamCityBuildQueue struct {
	Count int    `json:"count"`
	Href  string `json:"href"`
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
		}
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- buildQueueCount
}
