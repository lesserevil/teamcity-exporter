package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
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
	href   string                 `json:"href"`
	Builds []TeamCityBuildSummary `json:"build"`
}

type TeamCityBuildType struct {
	ID          string `json:"id"`
	href        string `json:"href"`
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
	ProjectID   string `json:"projectId"`
}

type TeamCityBuild struct {
	ID         int               `json:"id"`
	WaitReason string            `json:"waitReason"`
	href       string            `json:"href"`
	BuildType  TeamCityBuildType `json:"buildType"`
}

type TeamCityPool struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	href string `json:"href"`
}

type TeamCityAgent struct {
	ID   int          `json:"id"`
	href string       `json:"href"`
	Name string       `json:"name"`
	Pool TeamCityPool `json:"pool"`
}

type TeamCityAgents struct {
	Agents []TeamCityAgent `json:"agent"`
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
	r, err := u.Parse(route)
	u = *u.ResolveReference(r)
	logrus.Infof("url: %s", u.String())
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
	//decoder := json.NewDecoder(resp.Body)
	//if err := decoder.Decode(&v); err != nil {
	body, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &v); err != nil {
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

func (e *Exporter) GetCompatibleAgents(id int) (*TeamCityAgents, error) {
	var teamCityAgents *TeamCityAgents
	err := e.requestEndpoint(fmt.Sprintf("app/rest/agents?locator=compatible:(build:(id:%d))&fields=agent:(id,href,pool,name)", id), &teamCityAgents)
	if err != nil {
		logrus.Errorf("Can't get compatible agents: %s", err)
		return nil, err
	}
	return teamCityAgents, nil
}

func (e *Exporter) GetAgent(ID int) (*TeamCityAgent, error) {
	var teamCityAgent *TeamCityAgent
	err := e.requestEndpoint(fmt.Sprintf("app/rest/agent/id:%d", ID), &teamCityAgent)
	if err != nil {
		return nil, err
	}
	return teamCityAgent, nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	_, err := e.GetTeamCityServerInformation()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0.0,
		)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1.0,
	)
	bq, err := e.GetTeamCityBuildQueue()
	if err != nil {
		logrus.Errorf("Can't get build queue: %s", err)
		return
	}
	metrics := map[string]map[string]map[int]map[string]int{}
	metrics = make(map[string]map[string]map[int]map[string]int)
	//for each build in queue
	for i := 0; i < len(bq.Builds); i++ {
		b, err := e.GetTeamCityQueuedBuild(bq.Builds[i].ID)
		if err != nil {
			logrus.Errorf("Can't get queue build: %s", err)
			continue
		}
		//get reason
		reason := b.WaitReason
		if len(reason) == 0 {
			logrus.Warningf("Build has no reason: %+v", b)
			reason = reasonNoAgents
		}
		reason = strings.FieldsFunc(reason, func(c rune) bool { return c == ':' })[0] //strip off anything after a ":"
		project := b.BuildType.ProjectID
		//get list of compatible agents
		ca, err := e.GetCompatibleAgents(b.ID)
		if err != nil {
			logrus.Errorf("Can't get compatible agents: %s", err)
			continue
		}
		for _, agent := range ca.Agents {
			poolname := agent.Pool.Name
			if len(poolname) == 0 {
				poolname = "Default"
			}
			buildID := int(b.ID)
			//add metric to metric map for reason, pool, and project
			if _, found := metrics[reason]; !found {
				metrics[reason] = make(map[string]map[int]map[string]int)
			}
			if _, found := metrics[reason][project]; !found {
				metrics[reason][project] = make(map[int]map[string]int)
			}
			if _, found := metrics[reason][project][b.ID]; !found {
				metrics[reason][project][buildID] = make(map[string]int)
			}
			if _, found := metrics[reason][project][buildID][poolname]; !found {
				metrics[reason][project][buildID][poolname] = 0
			}
			metrics[reason][project][buildID][poolname] = 1
		}
	}

	logrus.Infof("metrics: %+v", metrics)

	//for each entry in metric map
	for reason, projmap := range metrics {
		for project, idmap := range projmap {
			for id, poolmap := range idmap {
				for poolname, count := range poolmap {
					//publish metric
					ch <- prometheus.MustNewConstMetric(
						buildQueueWaitCount, prometheus.GaugeValue, float64(count), reason, project, strconv.Itoa(id), poolname)
				}
			}
		}
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
}
