/*Copyright 2018 EPAM Systems
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

//ErrorLoggingLevel is integer representation of ERROR logging level
const ErrorLoggingLevel int = 40000

//indexType is type of index in ES
const indexType string = "log"

//nl is new line symbol
var nl = []byte("\n")

// ESClient interface
type ESClient interface {
	ListIndices() ([]Index, error)
	CreateIndex(name string) (*Response, error)
	IndexExists(name string) (bool, error)
	DeleteIndex(name string) (*Response, error)

	IndexLogs(launches []Launch) (*BulkResponse, error)
	DeleteLogs(ci *CleanIndex) (*Response, error)
	AnalyzeLogs(launches []Launch) ([]AnalysisResult, error)

	Healthy() bool

	createIndexIfNotExists(indexName string) error
	buildURL(pathElements ...string) string
}

// Response struct
type Response struct {
	Acknowledged bool `json:"acknowledged,omitempty"`
	Error        struct {
		RootCause []struct {
			Type   string `json:"type,omitempty"`
			Reason string `json:"reason,omitempty"`
		} `json:"root_cause,omitempty"`
		Type   string `json:"type,omitempty"`
		Reason string `json:"reason,omitempty"`
	} `json:"error,omitempty"`
	Status int `json:"status,omitempty"`
}

// BulkResponse struct
type BulkResponse struct {
	Took   int  `json:"took,omitempty"`
	Errors bool `json:"errors,omitempty"`
	Items  []struct {
		Index struct {
			Index   string `json:"_index,omitempty"`
			Type    string `json:"_type,omitempty"`
			ID      string `json:"_id,omitempty"`
			Version int    `json:"_version,omitempty"`
			Result  string `json:"result,omitempty"`
			Created bool   `json:"created,omitempty"`
			Status  int    `json:"status,omitempty"`
		} `json:"index,omitempty"`
	} `json:"items,omitempty"`
	Status int `json:"status,omitempty"`
}

// Launch struct
type Launch struct {
	LaunchID   string `json:"launchId,required" validate:"required"`
	Project    string `json:"project,required" validate:"required"`
	LaunchName string `json:"launchName,omitempty"`
	TestItems  []struct {
		TestItemID        string `json:"testItemId,required" validate:"required"`
		UniqueID          string `json:"uniqueId,required" validate:"required"`
		IsAutoAnalyzed    bool   `json:"isAutoAnalyzed,required" validate:"required"`
		IssueType         string `json:"issueType,omitempty"`
		OriginalIssueType string `json:"originalIssueType,omitempty"`
		Logs              []struct {
			LogID    string `json:"log_id,required" validate:"required"`
			LogLevel int    `json:"logLevel,omitempty"`
			Message  string `json:"message,required" validate:"required"`
		} `json:"logs,omitempty"`
	} `json:"testItems,omitempty"`
}

// Index struct
type Index struct {
	Health       string `json:"health,omitempty"`
	Status       string `json:"status,omitempty"`
	Index        string `json:"index,omitempty"`
	UUID         string `json:"uuid,omitempty"`
	Pri          string `json:"pri,omitempty"`
	Rep          string `json:"rep,omitempty"`
	DocsCount    string `json:"docs.count,omitempty"`
	DocsDeleted  string `json:"docs.deleted,omitempty"`
	StoreSize    string `json:"store.size,omitempty"`
	PriStoreSize string `json:"pri.store.size,omitempty"`
}

// SearchResult struct
type SearchResult struct {
	Took     int  `json:"took,omitempty"`
	TimedOut bool `json:"timed_out,omitempty"`
	Hits     struct {
		Total    int     `json:"total,omitempty"`
		MaxScore float64 `json:"max_score,omitempty"`
		Hits     []Hit   `json:"hits,omitempty"`
	} `json:"hits,omitempty"`
}

//Hit is a single result from search index
type Hit struct {
	Index  string  `json:"_index,omitempty"`
	Type   string  `json:"_type,omitempty"`
	ID     string  `json:"_id,omitempty"`
	Score  float64 `json:"_score,omitempty"`
	Source struct {
		TestItem   string `json:"test_item,omitempty"`
		IssueType  string `json:"issue_type,omitempty"`
		Message    string `json:"message,omitempty"`
		LogLevel   int    `json:"log_level,omitempty"`
		LaunchName string `json:"launch_name,omitempty"`
	} `json:"_source,omitempty"`
}

//AnalysisResult represents result of analyzes which is basically array of found matches (predicted issue type and ID of most relevant Test Item)
type AnalysisResult struct {
	TestItem     string `json:"test_item,omitempty"`
	IssueType    string `json:"issue_type,omitempty"`
	RelevantItem string `json:"relevant_item,omitempty"`
}

//CleanIndex is a request to clean index
type CleanIndex struct {
	IDs     []string `json:"ids,omitempty"`
	Project string   `json:"project,required" validate:"required"`
}

type client struct {
	hosts     []string
	re        *regexp.Regexp
	hc        *http.Client
	searchCfg *SearchConfig
}

// NewClient creates new ESClient
func NewClient(hosts []string, searchCfg *SearchConfig) ESClient {
	return &client{
		hosts:     hosts,
		searchCfg: searchCfg,
		hc:        &http.Client{},
	}
}

func (rs *Response) String() string {
	s, err := json.Marshal(rs)
	if err != nil {
		s = []byte{}
	}
	return fmt.Sprintf("%v", string(s))
}

//Healthy returns TRUE if cluster in operational state
func (c *client) Healthy() bool {
	var rs map[string]interface{}
	err := c.sendOpRequest(http.MethodGet, c.buildURL("_cluster/health"), &rs, nil)
	if nil != err {
		return false
	}
	status := rs["status"]
	return "yellow" == status || "green" == status
}

func (c *client) ListIndices() ([]Index, error) {
	url := c.buildURL("_cat", "indices?format=json")

	indices := []Index{}

	err := c.sendOpRequest(http.MethodGet, url, &indices)
	if err != nil {
		return nil, err
	}

	return indices, nil
}

func (c *client) CreateIndex(name string) (*Response, error) {
	body := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards": 1,
		},
		"mappings": map[string]interface{}{
			"log": map[string]interface{}{
				"properties": map[string]interface{}{
					"test_item": map[string]interface{}{
						"type": "keyword",
					},
					"issue_type": map[string]interface{}{
						"type": "keyword",
					},
					"message": map[string]interface{}{
						"type": "keyword",
					},
					"log_level": map[string]interface{}{
						"type": "integer",
					},
					"launch_name": map[string]interface{}{
						"type": "keyword",
					},
					"unique_id": map[string]interface{}{
						"type": "keyword",
					},
					"is_auto_analyzed": map[string]interface{}{
						"type": "keyword",
					},
				},
			},
		},
	}

	url := c.buildURL(name)

	rs := &Response{}

	return rs, c.sendOpRequest(http.MethodPut, url, rs, body)
}

func (c *client) IndexExists(name string) (bool, error) {
	url := c.buildURL(name)

	httpClient := &http.Client{}
	rs, err := httpClient.Head(url)
	if err != nil {
		return false, err
	}

	return rs.StatusCode == http.StatusOK, nil
}

func (c *client) DeleteIndex(name string) (*Response, error) {
	url := c.buildURL(name)
	rs := &Response{}
	return rs, c.sendOpRequest(http.MethodDelete, url, rs)
}

func (c *client) DeleteLogs(ci *CleanIndex) (*Response, error) {
	url := c.buildURL("_bulk")
	rs := &Response{}
	bodies := make([]interface{}, len(ci.IDs))
	for i, id := range ci.IDs {
		bodies[i] = map[string]interface{}{
			"delete": map[string]interface{}{
				"_id":    id,
				"_index": getIndexName(ci.Project),
				"_type":  indexType,
			},
		}
	}
	return rs, c.sendOpRequest(http.MethodPost, url, rs, bodies...)
}

func (c *client) IndexLogs(launches []Launch) (*BulkResponse, error) {

	var bodies []interface{}

	for _, lc := range launches {
		err := c.createIndexIfNotExists(getIndexName(lc.Project))
		if nil != err {
			return nil, err
		}
		for _, ti := range lc.TestItems {
			for _, l := range ti.Logs {

				op := map[string]interface{}{
					"index": map[string]interface{}{
						"_id":    l.LogID,
						"_index": getIndexName(lc.Project),
						"_type":  indexType,
					},
				}

				bodies = append(bodies, op)

				body := map[string]interface{}{
					"launch_name":      lc.LaunchName,
					"test_item":        ti.TestItemID,
					"unique_id":        ti.UniqueID,
					"is_auto_analyzed": ti.IsAutoAnalyzed,
					"issue_type":       ti.IssueType,
					"log_level":        l.LogLevel,
					"message":          l.Message,
				}

				bodies = append(bodies, body)
			}
		}
	}

	rs := &BulkResponse{}

	if len(bodies) == 0 {
		return rs, nil
	}

	url := c.buildURL("_bulk")

	return rs, c.sendOpRequest(http.MethodPut, url, rs, bodies...)
}

func (c *client) AnalyzeLogs(launches []Launch) ([]AnalysisResult, error) {
	result := []AnalysisResult{}
	for _, lc := range launches {
		url := c.buildURL(getIndexName(lc.Project), "log", "_search")

		for _, ti := range lc.TestItems {
			issueTypes := make(map[string]*score)

			for _, l := range ti.Logs {

				query := c.buildQuery(lc.LaunchName, ti.UniqueID, l.Message)

				rs := &SearchResult{}
				err := c.sendOpRequest(http.MethodGet, url, rs, query)
				if err != nil {
					return nil, err
				}

				calculateScores(rs, 10, issueTypes)
			}

			var predictedIssueType string

			if len(issueTypes) > 0 {
				max := 0.0
				for k, v := range issueTypes {
					if v.score > max {
						max = v.score
						predictedIssueType = k
					}
				}
			}
			if "" != predictedIssueType {
				result = append(result, AnalysisResult{
					TestItem:     ti.TestItemID,
					RelevantItem: issueTypes[predictedIssueType].mrHit.Source.TestItem,
					IssueType:    predictedIssueType,
				})
			}

		}
	}

	return result, nil
}

func (c *client) createIndexIfNotExists(indexName string) error {
	exists, err := c.IndexExists(indexName)
	if err != nil {
		return errors.Wrap(err, "Cannot check ES index exists")
	}
	if !exists {
		_, err = c.CreateIndex(indexName)
	}
	return errors.Wrap(err, "Cannot create ES index")
}

func (c *client) buildURL(pathElements ...string) string {
	return c.hosts[0] + "/" + strings.Join(pathElements, "/")
}

func (c *client) buildQuery(launchName, uniqueID, logMessage string) interface{} {
	return map[string]interface{}{
		"size": 10,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must_not": map[string]interface{}{
					"wildcard": map[string]interface{}{
						"issue_type": "TI*",
					},
				},
				"must": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"log_level": ErrorLoggingLevel,
						},
					},
					map[string]interface{}{
						"exists": map[string]interface{}{
							"field": "issue_type",
						},
					},
					map[string]interface{}{
						"term": map[string]interface{}{
							"message": logMessage,
						},
					},
				},
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{
						"launch_name": map[string]interface{}{
							"value": launchName,
							"boost": math.Abs(c.searchCfg.BoostLaunch),
						},
					}},
					{"term": map[string]interface{}{
						"unique_id": map[string]interface{}{
							"value": uniqueID,
							"boost": math.Abs(c.searchCfg.BoostUniqueID),
						},
					}},
					{"term": map[string]interface{}{
						"is_auto_analyzed": map[string]interface{}{
							"value": strconv.FormatBool(c.searchCfg.BoostAA < 0),
							"boost": math.Abs(c.searchCfg.BoostAA),
						},
					}},
				},
			},
		},
	}
}

//score represents total score for defect type
//mrHit is hit with highest score found (most relevant hit)
type score struct {
	score float64
	mrHit Hit
}

func calculateScores(rs *SearchResult, k int, scores map[string]*score) {
	if rs.Hits.Total > 0 {
		n := len(rs.Hits.Hits)
		if n < k {
			k = n
		}
		totalScore := 0.0
		hits := rs.Hits.Hits[:k]
		// Two iterations over hits needed
		// to achieve stable prediction
		for _, h := range hits {
			totalScore += h.Score

			//find the hit with highest score for each defect type.
			//item from the hit will be used as most relevant of request is analysed successfully
			if typeScore, ok := scores[h.Source.IssueType]; ok {
				if h.Score > typeScore.mrHit.Score {
					typeScore.mrHit = h
				}
			} else {
				scores[h.Source.IssueType] = &score{mrHit: h}
			}

		}
		for _, h := range hits {
			typeScore, ok := scores[h.Source.IssueType]
			currScore := h.Score / totalScore
			if ok {
				typeScore.score += currScore
			} else {
				//should never happen
				log.Errorf("Internal error during AA score calculation. Missed issue type: %s", h.Source.IssueType)
				scores[h.Source.IssueType] = &score{currScore, h}
			}
		}
	}
}

func (c *client) sendOpRequest(method, url string, response interface{}, bodies ...interface{}) error {
	rs, err := c.sendRequest(method, url, bodies...)
	if err != nil {

		return err
	}

	err = json.Unmarshal(rs, &response)
	if err != nil {
		return errors.Wrap(err, "Cannot unmarshal ES OP response")
	}

	return nil
}

func (c *client) sendRequest(method, url string, bodies ...interface{}) ([]byte, error) {
	var rdr io.Reader

	if len(bodies) > 0 {
		buff := bytes.NewBuffer([]byte{})
		for _, body := range bodies {
			if err := writeBody(buff, body); nil != err {
				log.Error(err)

			}
		}
		rdr = buff
	}

	rq, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return nil, errors.Wrap(err, "Cannot build request to ES")
	}
	rq.Header.Set("Content-Type", "application/json")

	rs, err := c.hc.Do(rq)
	if err != nil {
		log.Errorf("Cannot send request to ES: %s", err.Error())

		return nil, errors.Wrap(err, "Cannot send request to ES")
	}
	defer rs.Body.Close()

	rsBody, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		log.Errorf("Cannot ES response: %s", err.Error())
		return nil, errors.Wrap(err, "Cannot read ES response")
	}

	log.Infof("ES responded with %d status code", rs.StatusCode)
	if rs.StatusCode > http.StatusCreated && rs.StatusCode < http.StatusNotFound {
		body := string(rsBody)
		log.Errorf("ES communication error. Status code %d, Body %s", rs.StatusCode, body)
		return nil, errors.New(body)
	}

	return rsBody, nil
}

func getIndexName(name string) string {
	return "equalindex-" + name
}

func writeBody(buff io.Writer, body interface{}) error {
	rqBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if _, err = buff.Write(rqBody); nil != err {
		log.Error(err)
	}
	if _, err := buff.Write(nl); nil != err {
		log.Error(err)
	}
	return nil
}
