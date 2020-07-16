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

type TeamCityBuilds struct {
	Count  int             `json:"count"`
	href   string          `json:"href"`
	Builds []TeamCityBuild `json:"build"`
}

type TeamCityBuildQueue TeamCityBuilds

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
	Agent      TeamCityAgent     `json:"agent"`
}

type TeamCityPool struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	href string `json:"href"`
}

type TeamCityInfo struct {
	Status bool `json:"status"`
}

type TeamCityProperties map[string]string

func (p *TeamCityProperties) UnmarshalJSON(b []byte) error {
	var s map[string]interface{}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	var ptmp = make(TeamCityProperties)

	var props = s["property"].([]interface{})

	for _, prop := range props {
		var proptmp = prop.(map[string]interface{})
		var name = proptmp["name"].(string)
		var value = proptmp["value"].(string)
		ptmp[name] = value
	}

	*p = make(TeamCityProperties)
	*p = ptmp

	return nil
}

type TeamCityAgent struct {
	ID             int                `json:"id"`
	href           string             `json:"href"`
	Name           string             `json:"name"`
	Pool           TeamCityPool       `json:"pool"`
	EnabledInfo    TeamCityInfo       `json:"enabledInfo"`
	AuthorizedInfo TeamCityInfo       `json:"authorizedInfo"`
	Connected      bool               `json:"connected"`
	Properties     TeamCityProperties `json:"properties"`
}

type TeamCityAgents struct {
	Agents []TeamCityAgent `json:"agent"`
}

type TeamCityProject struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ParentProjectID string `json:"parentProjectId"`
	href            string `json:"href"`
}

func NewExporter(config *Config) *Exporter {
	return &Exporter{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (e *Exporter) requestEndpoint(route string, v interface{}) error {
	u := *e.config.apiEndpointUrl
	r, err := u.Parse(route)
	u = *u.ResolveReference(r)
	logrus.Debugf("url: %s", u.String())
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
	err := e.requestEndpoint("app/rest/buildQueue/?fields=count,href,build:(id,waitReason,href,buildType:(id,href,name,projectName,projectId))", &teamCityBuildQueue)
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
	err := e.requestEndpoint(fmt.Sprintf("app/rest/agents?locator=compatible:(build:(id:%d))&fields=agent:(id,href,pool,name,properties(property))", id), &teamCityAgents)
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

func (e *Exporter) GetAllAgents() (*TeamCityAgents, error) {
	var agents *TeamCityAgents
	err := e.requestEndpoint("app/rest/agents?locator=authorized:any,defaultFilter:false&fields=agent:(id,href,enabledInfo,authorizedInfo,connected,pool,name,properties(property))", &agents)
	if err != nil {
		return nil, err
	}
	return agents, nil
}

func (e *Exporter) GetRunningBuilds() (*TeamCityBuilds, error) {
	var builds *TeamCityBuilds
	err := e.requestEndpoint("app/rest/builds?locator=running:true&fields=count,href,build(buildType,agent:(id,href,name,pool))", &builds)
	if err != nil {
		return nil, err
	}
	return builds, nil
}

func (e *Exporter) GetTopProject(ProjectID string, projects map[string]string) (*string, error) {
	if _, found := projects[ProjectID]; found {
		parent := projects[ProjectID]
		if parent == "_Root" {
			return &ProjectID, nil
		} else {
			return e.GetTopProject(parent, projects)
		}
	}
	var parent *TeamCityProject
	err := e.requestEndpoint(fmt.Sprintf("app/rest/projects/id:%v", ProjectID), &parent)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("parent: %+v", parent)
	ParentID := parent.ParentProjectID
	projects[ProjectID] = parent.ParentProjectID
	if ParentID == "_Root" {
		return &ProjectID, nil
	} else {
		return e.GetTopProject(ParentID, projects)
	}
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	var projects map[string]string
	projects = make(map[string]string)
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
	metrics := map[string]map[string]map[int]map[string]map[bool]map[bool]map[bool]int{}
	metrics = make(map[string]map[string]map[int]map[string]map[bool]map[bool]map[bool]int)
	//for each build in queue
	for _, b := range bq.Builds {
		logrus.Debugf("b: %+v", b)
		//get reason
		reason := b.WaitReason
		if len(reason) == 0 {
			logrus.Infof("Build has no reason: %+v", b)
			reason = reasonDefault
		}
		reason = strings.FieldsFunc(reason, func(c rune) bool { return c == ':' })[0] //strip off anything after a ":"
		reason = strings.FieldsFunc(reason, func(c rune) bool { return c == ',' })[0] //strip off anything after a ","
		for len(strings.FieldsFunc(reason, func(c rune) bool { return c == '"' })) >= 2 {
			var tmp = strings.FieldsFunc(reason, func(c rune) bool { return c == '"' })
			reason = tmp[0] + strings.Join(tmp[2:], "\"")
		}
		tmpproj, err := e.GetTopProject(b.BuildType.ProjectID, projects)
		if err != nil {
			logrus.Errorf("Cant get project info: %s", err)
			continue
		}
		project := *tmpproj
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
			//add metric to metric map for id, reason, pool, project, and allowed OS
			if _, found := metrics[reason]; !found {
				metrics[reason] = make(map[string]map[int]map[string]map[bool]map[bool]map[bool]int)
			}
			if _, found := metrics[reason][project]; !found {
				metrics[reason][project] = make(map[int]map[string]map[bool]map[bool]map[bool]int)
			}
			if _, found := metrics[reason][project][b.ID]; !found {
				metrics[reason][project][buildID] = make(map[string]map[bool]map[bool]map[bool]int)
			}
			if _, found := metrics[reason][project][buildID][poolname]; !found {
				metrics[reason][project][buildID][poolname] = make(map[bool]map[bool]map[bool]int)
			}
			var winOk = false
			var linOk = false
			var macOk = false
			for _, a := range ca.Agents {
				winOk = winOk || strings.Contains(a.Properties["teamcity.agent.jvm.os.name"], "Windows")
				linOk = linOk || strings.Contains(a.Properties["teamcity.agent.jvm.os.name"], "Linux")
				macOk = macOk || strings.Contains(a.Properties["teamcity.agent.jvm.os.name"], "Mac")
			}
			metrics[reason][project][buildID][poolname][winOk] = make(map[bool]map[bool]int)
			metrics[reason][project][buildID][poolname][winOk][linOk] = make(map[bool]int)
			metrics[reason][project][buildID][poolname][winOk][linOk][macOk] = 1
		}
	}

	logrus.Debugf("metrics: %+v", metrics)

	//for each entry in metric map
	for reason, projmap := range metrics {
		for project, idmap := range projmap {
			for id, poolmap := range idmap {
				for poolname, winmap := range poolmap {
					for win, linmap := range winmap {
						for lin, macmap := range linmap {
							for mac, count := range macmap {
								//publish metric
								ch <- prometheus.MustNewConstMetric(
									buildQueueWaitCount, prometheus.GaugeValue, float64(count), reason, project, strconv.Itoa(id), poolname,
									strconv.FormatBool(win), strconv.FormatBool(lin), strconv.FormatBool(mac))
							}
						}
					}
				}
			}
		}
	}

	var agentInfo map[string]map[string]map[string]map[string]map[string]map[string]map[string]int
	agentInfo = make(map[string]map[string]map[string]map[string]map[string]map[string]map[string]int)

	var allAgents, _ = e.GetAllAgents()
	var runningBuilds, _ = e.GetRunningBuilds()
	var runningAgents map[int]TeamCityAgent
	runningAgents = make(map[int]TeamCityAgent)
	for _, build := range runningBuilds.Builds {
		runningAgents[build.Agent.ID] = build.Agent
	}

	for _, agent := range allAgents.Agents {

		var pool = agent.Pool.Name
		var agentos = "Other"
		if _, ok := agent.Properties["system.feature.windows.version"]; ok {
			agentos = "Windows"
		} else if _, ok := agent.Properties["system.feature.linux.version"]; ok {
			agentos = "Linux"
		}
		var enabled = strconv.FormatBool(agent.EnabledInfo.Status)
		var authorized = strconv.FormatBool(agent.AuthorizedInfo.Status)
		var connected = strconv.FormatBool(agent.Connected)
		var busy = strconv.FormatBool(false)
		var project = ""
		if _, found := runningAgents[agent.ID]; found {
			busy = strconv.FormatBool(true)
		}
		if busy == "true" {
			for _, b := range runningBuilds.Builds {
				if agent.ID == b.Agent.ID {
					var tmpproj, _ = e.GetTopProject(b.BuildType.ProjectID, projects)
					project = *tmpproj
				}
			}
		}
		logrus.Debugf(project)

		if _, found := agentInfo[pool]; !found {
			agentInfo[pool] = make(map[string]map[string]map[string]map[string]map[string]map[string]int)
		}
		if _, found := agentInfo[pool][agentos]; !found {
			agentInfo[pool][agentos] = make(map[string]map[string]map[string]map[string]map[string]int)
		}
		if _, found := agentInfo[pool][agentos][enabled]; !found {
			agentInfo[pool][agentos][enabled] = make(map[string]map[string]map[string]map[string]int)
		}
		if _, found := agentInfo[pool][agentos][enabled][authorized]; !found {
			agentInfo[pool][agentos][enabled][authorized] = make(map[string]map[string]map[string]int)
		}
		if _, found := agentInfo[pool][agentos][enabled][authorized][connected]; !found {
			agentInfo[pool][agentos][enabled][authorized][connected] = make(map[string]map[string]int)
		}
		if _, found := agentInfo[pool][agentos][enabled][authorized][connected][project]; !found {
			agentInfo[pool][agentos][enabled][authorized][connected][project] = make(map[string]int)
		}
		if _, found := agentInfo[pool][agentos][enabled][authorized][connected][project][busy]; !found {
			agentInfo[pool][agentos][enabled][authorized][connected][project][busy] = 0
		}
		agentInfo[pool][agentos][enabled][authorized][connected][project][busy]++
	}

	for poolname, osmap := range agentInfo {
		for osname, enabledmap := range osmap {
			for enabled, authorizedmap := range enabledmap {
				for authorized, connectedmap := range authorizedmap {
					for connected, projectmap := range connectedmap {
						for project, busymap := range projectmap {
							for busy, count := range busymap {
								ch <- prometheus.MustNewConstMetric(
									agentInfoCount, prometheus.GaugeValue, float64(count), poolname, osname, enabled, authorized, connected, project, busy)
							}
						}
					}
				}
			}
		}
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
}
