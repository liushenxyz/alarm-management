package connector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	netUrl "net/url"
	"strings"
)

type Zabbix struct {
	url   string
	token string
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func GeneratePosts(queryString, delay string) string {
	posts := fmt.Sprintf(`{"query":{"bool":{"must":[{"query_string":{"query":"%s"}},{"range":{"@timestamp":{"format":"strict_date_optional_time","gte":"now-%s","lte":"now"}}}]}},"size":0}`, queryString, delay)
	return posts
}

type Item struct {
	ItemID string `json:"itemid"`
	HostID string `json:"hostid"`
	Name   string `json:"name"`
	Key    string `json:"key_"`
	Delay  string `json:"delay"`
	Url    string `json:"url"`
	Posts  string `json:"posts"`
}

func (i *Item) GetQueryString() string {
	s := i.Posts
	var data map[string]interface{}
	err := json.Unmarshal([]byte(s), &data)
	if err != nil {
		return "无法解析 JSON"
	}

	queryString, ok := data["query"].(map[string]interface{})["bool"].(map[string]interface{})["must"].([]interface{})[0].(map[string]interface{})["query_string"].(map[string]interface{})["query"].(string)
	if !ok {
		return "无法找到 query_string 的值"
	}
	return queryString
}

func (i *Item) GetIndex() string {
	s := i.Url
	// Find the last index of "/"
	lastSlashIndex := strings.LastIndex(i.Url, "/")
	// Find the second last index of "/"
	secondLastSlashIndex := strings.LastIndex(s[:lastSlashIndex], "/")
	// Extract the substring between the second last and last slash
	index := s[secondLastSlashIndex+1 : lastSlashIndex]
	return index
}

func (i *Item) GetElasticsearch() string {
	u, err := netUrl.Parse(i.Url)
	if err != nil {
		return "无法解析 URL"
	}
	elasticsearch := fmt.Sprintf("%s//%s", u.Scheme, u.Host)
	return elasticsearch
}

type Trigger struct {
	TriggerID   string `json:"triggerid"`
	Expression  string `json:"expression"`
	Description string `json:"description"`
}

func (t *Trigger) GetThreshold() string {
	// 查找 "}" 的索引位置
	index := strings.Index(t.Expression, "}")
	if index == -1 {
		return ""
	}
	// 截取 "}" 之后的部分
	threshold := t.Expression[index+1:]
	return threshold
}

type Host struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
}

func NewZabbix(url, token string) *Zabbix {
	return &Zabbix{url, token}
}

func (z *Zabbix) RequestApi(payload map[string]interface{}) ([]byte, error) {

	url := z.url
	method := "GET"
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("JSON编码失败：%s", err.Error())
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败：%s", err.Error())
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败：%s", err.Error())
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败：%s", err.Error())
	}

	return responseBody, nil
}

func (z *Zabbix) CreateItem(name, key, hostid, delay, username, password, url, posts string) (string, error) {

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "item.create",
		"params": map[string]interface{}{
			"type":           19,
			"name":           name,
			"key_":           key,
			"hostid":         hostid,
			"delay":          delay,
			"value_type":     3,
			"output_format":  1,
			"authtype":       1,
			"username":       username,
			"password":       password,
			"timeout":        "30s",
			"url":            url,
			"posts":          posts,
			"post_type":      2,
			"request_method": 0,
			"headers": map[string]string{
				"Content-Type": "application/json",
			},
			"preprocessing": []map[string]string{
				{
					"type":                 "12",
					"params":               "$.body.hits.total.value",
					"error_handler":        "0",
					"error_handler_params": "",
				},
			},
			"tags": []map[string]string{
				{"tag": "logs", "value": "alert"},
			},
		},
		"id":   1,
		"auth": z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return "", fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError       `json:"error"`
		Result map[string][]string `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return "", fmt.Errorf("创建监控项失败：%s", response.Error.Data)
	}

	return response.Result["itemids"][0], nil
}

func (z *Zabbix) GetItemByName(itemName string) (Item, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "item.get",
		"params": map[string]interface{}{
			"filter": map[string]interface{}{
				"name": []string{itemName},
			},
			"tags": []map[string]string{
				{"tag": "logs", "operator": "4"},
			},
		},
		"id":   1,
		"auth": z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return Item{}, fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError `json:"error"`
		Result []Item        `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return Item{}, fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return Item{}, fmt.Errorf("获取监控项失败：%s", response.Error.Data)
	}

	if len(response.Result) > 0 {
		return response.Result[0], nil
	}

	return Item{}, fmt.Errorf("监控项不存在：%s", itemName)

}

func (z *Zabbix) GetItems() ([]Item, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "item.get",
		"params": map[string]interface{}{
			"tags": []map[string]string{
				{"tag": "logs", "operator": "4"},
			},
		},
		"id":   1,
		"auth": z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return []Item{}, fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError `json:"error"`
		Result []Item        `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return []Item{}, fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return []Item{}, fmt.Errorf("获取监控项失败：%s", response.Error.Data)
	}

	return response.Result, nil

}

func (z *Zabbix) DeleteItemByID(itemId string) (string, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "item.delete",
		"params":  []string{itemId},
		"id":      1,
		"auth":    z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return "", fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError       `json:"error"`
		Result map[string][]string `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return "", fmt.Errorf("删除监控项失败：%s", response.Error.Data)
	}

	if len(response.Result["itemids"]) > 0 {
		return response.Result["itemids"][0], nil
	}

	return "", fmt.Errorf("监控项不存在：%s", itemId)
}

func (z *Zabbix) GetHostByName(hostName string) (Host, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "host.get",
		"params": map[string]interface{}{
			"filter": map[string]interface{}{
				"host": []string{hostName},
			},
		},
		"id":   1,
		"auth": z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return Host{}, fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError `json:"error"`
		Result []Host        `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return Host{}, fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return Host{}, fmt.Errorf("获取主机失败：%s", response.Error.Data)
	}

	if len(response.Result) > 0 {
		return response.Result[0], nil
	}

	return Host{}, fmt.Errorf("主机不存在：%s", hostName)
}

func (z *Zabbix) CreateTrigger(hostName, itemName, itemKey, threshold string) (string, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "trigger.create",
		"params": map[string]interface{}{
			"expression":  fmt.Sprintf("last(/%s/%s,#3)%s", hostName, itemKey, threshold),
			"description": itemName,
			"priority":    "5",
			"tags": []map[string]string{
				{"tag": "logs", "value": "alert"},
			},
		},
		"id":   1,
		"auth": z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return "", fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError       `json:"error"`
		Result map[string][]string `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return "", fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return "", fmt.Errorf("创建触发器失败：%s", response.Error.Data)
	}

	return response.Result["triggerids"][0], nil
}

func (z *Zabbix) GetTriggerByName(triggerName string) (Trigger, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "trigger.get",
		"params": map[string]interface{}{
			"filter": map[string]interface{}{
				"description": []string{triggerName},
			},
			"tags": []map[string]string{
				{"tag": "logs", "operator": "4"},
			},
		},
		"id":   1,
		"auth": z.token,
	}

	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return Trigger{}, fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError `json:"error"`
		Result []Trigger     `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return Trigger{}, fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return Trigger{}, fmt.Errorf("获取触发器失败：%s", response.Error.Data)
	}

	if len(response.Result) > 0 {
		return response.Result[0], nil
	}

	return Trigger{}, fmt.Errorf("触发器不存在：%s", triggerName)

}

func (z *Zabbix) GetTriggerByID(triggerID string) (Trigger, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "trigger.get",
		"params": map[string]interface{}{
			"triggerids": triggerID,
		},
		"id":   1,
		"auth": z.token,
	}
	responseBody, err := z.RequestApi(payload)
	if err != nil {
		return Trigger{}, fmt.Errorf("请求ZabbixAPI失败：%s", err.Error())
	}

	var response struct {
		Error  ResponseError `json:"error"`
		Result []Trigger     `json:"result"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return Trigger{}, fmt.Errorf("解析响应失败：%s", err.Error())
	}

	if response.Error.Message != "" {
		return Trigger{}, fmt.Errorf("获取触发器失败：%s", response.Error.Data)
	}

	if len(response.Result) > 0 {
		return response.Result[0], nil
	}

	return Trigger{}, fmt.Errorf("触发器不存在：%s", triggerID)
}
