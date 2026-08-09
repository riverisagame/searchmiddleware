package zinc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"searchmiddleware/internal/config"
)

type Client struct {
	username       string
	password       string
	clusters       map[string]*Cluster
	defaultCluster string
	mu             sync.RWMutex
	httpClient     *http.Client
}

type Cluster struct {
	Name   string
	URLs   []string
	Index  int
	mu     sync.Mutex
	Health map[string]bool
}

func NewClient(cfg *config.ZincConfig) *Client {
	c := &Client{
		clusters:       make(map[string]*Cluster),
		defaultCluster: cfg.Default,
		username:       cfg.Username,
		password:       cfg.Password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for name, urls := range cfg.Clusters {
		c.clusters[name] = &Cluster{
			Name:   name,
			URLs:   urls,
			Health: make(map[string]bool),
		}
		for _, u := range urls {
			c.clusters[name].Health[u] = true
		}
		go c.healthCheckLoop(c.clusters[name])
	}

	return c
}

func (c *Client) getCluster(name string) *Cluster {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if name == "" {
		name = c.defaultCluster
	}
	return c.clusters[name]
}

func (c *Client) healthCheckLoop(cl *Cluster) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		cl.mu.Lock()
		for _, u := range cl.URLs {
			healthy := c.ping(u)
			cl.Health[u] = healthy
		}
		cl.mu.Unlock()
	}
}

func (c *Client) ping(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url+"/healthz", nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (c *Client) getHealthyURL(clusterName string) (string, error) {
	cl := c.getCluster(clusterName)
	if cl == nil {
		return "", fmt.Errorf("cluster not found: %s", clusterName)
	}

	cl.mu.Lock()
	defer cl.mu.Unlock()

	// 实际探测轮询：从上次位置开始逐个尝试，不依赖缓存（缓存仅用于展示）
	for i := 0; i < len(cl.URLs); i++ {
		idx := (cl.Index + i) % len(cl.URLs)
		url := cl.URLs[idx]
		if c.ping(url) {
			cl.Health[url] = true
			cl.Index = (idx + 1) % len(cl.URLs)
			return url, nil
		}
		cl.Health[url] = false
	}

	return "", fmt.Errorf("no healthy nodes in cluster: %s", clusterName)
}

func (c *Client) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	return req, nil
}

func (c *Client) Search(index string, body map[string]interface{}, clusterName string) (map[string]interface{}, error) {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return nil, err
	}

	data, _ := json.Marshal(body)
	req, err := c.newRequest("POST", url+"/es/"+index+"/_search", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if resp.StatusCode >= 500 {
			c.markUnhealthy(url)
		}
		return nil, fmt.Errorf("zinc search error %d: %s", resp.StatusCode, truncate(string(bodyBytes), 300))
	}

	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	return result, nil
}

func (c *Client) Bulk(index string, docs []map[string]interface{}, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	for _, doc := range docs {
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": index,
			},
		}
		if id, ok := doc["_id"]; ok {
			meta["index"].(map[string]interface{})["_id"] = id
		}
		metaBytes, _ := json.Marshal(meta)
		docBytes, _ := json.Marshal(doc)
		buf.Write(metaBytes)
		buf.WriteByte('\n')
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	req, err := c.newRequest("POST", url+"/es/"+index+"/_bulk", &buf)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if resp.StatusCode >= 500 {
			c.markUnhealthy(url)
		}
		return fmt.Errorf("bulk error %d: %s", resp.StatusCode, truncate(string(bodyBytes), 300))
	}

	// bulk 200 但逐条失败：解析响应体检查 errors 标志
	var bulkResp struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Status int         `json:"status"`
				Error  interface{} `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}
	if err := json.Unmarshal(bodyBytes, &bulkResp); err == nil && bulkResp.Errors {
		for _, item := range bulkResp.Items {
			if item.Index.Status >= 400 {
				return fmt.Errorf("bulk partial failure: status %d, error %v", item.Index.Status, item.Index.Error)
			}
		}
	}

	return nil
}

func (c *Client) CreateIndex(index string, mapping map[string]interface{}, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}

	body := map[string]interface{}{
		"mappings": mapping,
	}
	data, _ := json.Marshal(body)

	req, err := c.newRequest("PUT", url+"/es/"+index, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 400 {
		c.markUnhealthy(url)
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index error: %d - %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func (c *Client) DeleteIndex(index string, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}

	req, err := c.newRequest("DELETE", url+"/es/"+index, nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) AliasSwap(addMap, removeMap map[string][]string, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}

	actions := []map[string]interface{}{}
	for alias, indexes := range addMap {
		for _, idx := range indexes {
			actions = append(actions, map[string]interface{}{
				"add": map[string]interface{}{
					"index": idx,
					"alias": alias,
				},
			})
		}
	}
	for alias, indexes := range removeMap {
		for _, idx := range indexes {
			actions = append(actions, map[string]interface{}{
				"remove": map[string]interface{}{
					"index": idx,
					"alias": alias,
				},
			})
		}
	}

	body := map[string]interface{}{"actions": actions}
	data, _ := json.Marshal(body)

	req, err := c.newRequest("POST", url+"/es/_aliases", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		c.markUnhealthy(url)
		return fmt.Errorf("alias error: %d", resp.StatusCode)
	}
	return nil
}

// Refresh 强制刷新索引（NRT 可见性，SUG-003 规避）：POST /es/{index}/_refresh
func (c *Client) Refresh(index, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}
	req, err := c.newRequest("POST", url+"/es/"+index+"/_refresh", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		c.markUnhealthy(url)
		return fmt.Errorf("refresh error: %d", resp.StatusCode)
	}
	return nil
}

// GetAlias 返回 alias 名下所有索引（ES 语义）。
// 注意：Zinc 的 GET /es/{alias}/_alias 按 alias 名查询恒返回空（BUG-010 提报中），
// 故改调全量 GET /es/_alias 后本地过滤，保证按 alias 名查询可用。
func (c *Client) GetAlias(alias string, clusterName string) (map[string]interface{}, error) {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest("GET", url+"/es/_alias", nil)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, err
	}

	// 过滤：仅保留含该 alias 的索引（ES 语义：{索引名: {"aliases": {alias: ...}}})
	filtered := make(map[string]interface{})
	for idxName, meta := range result {
		aliases, ok := meta.(map[string]interface{})["aliases"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, has := aliases[alias]; has {
			filtered[idxName] = meta
		}
	}
	return filtered, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (c *Client) markUnhealthy(url string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, cl := range c.clusters {
		cl.mu.Lock()
		cl.Health[url] = false
		cl.mu.Unlock()
	}
}

func (c *Client) HealthCheck(clusterName string) map[string]bool {
	cl := c.getCluster(clusterName)
	if cl == nil {
		return nil
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	result := make(map[string]bool)
	for u, h := range cl.Health {
		result[u] = h
	}
	return result
}

func (c *Client) ScrollSearch(index string, body map[string]interface{}, clusterName string) ([]map[string]interface{}, error) {
	body["size"] = 1000
	body["scroll"] = "1m"

	var allDocs []map[string]interface{}
	scrollID := ""

	for {
		if scrollID != "" {
			body = map[string]interface{}{
				"scroll":    "1m",
				"scroll_id": scrollID,
			}
		}

		result, err := c.Search(index, body, clusterName)
		if err != nil {
			return nil, err
		}

		hits := result["hits"].(map[string]interface{})["hits"].([]interface{})
		if len(hits) == 0 {
			break
		}

		for _, hit := range hits {
			h := hit.(map[string]interface{})
			source := h["_source"].(map[string]interface{})
			source["_id"] = h["_id"]
			allDocs = append(allDocs, source)
		}

		scrollID = result["_scroll_id"].(string)
	}

	c.ClearScroll(scrollID, clusterName)
	return allDocs, nil
}

// DeleteDoc 按 _id 删除文档
func (c *Client) DeleteDoc(index, id, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}
	req, err := c.newRequest("DELETE", url+"/es/"+index+"/_doc/"+id, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		c.markUnhealthy(url)
		return fmt.Errorf("delete doc error: %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) UpdateMapping(index string, mapping map[string]interface{}, clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}
	// Zinc PUT /api/:target/_mapping 接受 {"properties": {...}} 格式
	body := mapping
	data, _ := json.Marshal(body)
	req, err := c.newRequest("PUT", url+"/api/"+index+"/_mapping", bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if resp.StatusCode >= 500 {
			c.markUnhealthy(url)
		}
		return fmt.Errorf("update mapping error %d: %s", resp.StatusCode, truncate(string(bodyBytes), 300))
	}
	return nil
}

// ReloadSynonym 触发 Zinc 重载同义词词典（REQ-002）
func (c *Client) ReloadSynonym(clusterName string) error {
	url, err := c.getHealthyURL(clusterName)
	if err != nil {
		return err
	}
	req, err := c.newRequest("POST", url+"/api/_reload/synonym", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.markUnhealthy(url)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		c.markUnhealthy(url)
		return fmt.Errorf("reload synonym error: %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reload synonym error %d: %s", resp.StatusCode, truncate(string(bodyBytes), 200))
	}
	return nil
}

func (c *Client) ClearScroll(scrollID, clusterName string) {
	url, _ := c.getHealthyURL(clusterName)
	body := map[string]interface{}{"scroll_id": []string{scrollID}}
	data, _ := json.Marshal(body)
	req, _ := c.newRequest("DELETE", url+"/_search/scroll", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	c.httpClient.Do(req)
}
