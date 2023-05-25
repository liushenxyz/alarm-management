package main

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"gin-zabbix/configs"
	"gin-zabbix/connector"
	"gin-zabbix/docs"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"os"
	"time"
)

// @BasePath /api/v1
// @Title Log Alarm Management Service
// @version 1.0
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @securityDefinitions.basic  BasicAuth

// HealthCheck
// @Summary Health Check
// @Schemes http
// @Description 健康检查
// @Tags monitor
// @Accept json
// @Produce json
// @Success 200 {string} Health Check
// @Router /monitor/health_check [get]
func HealthCheck(c *gin.Context) {
	config := c.MustGet("config").(configs.Config)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: tr,
	}
	// 检查Zabbix服务是否可用
	_, err := client.Get(config.Zabbix.Url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}
	// 检查Elasticsearch服务是否可用
	_, err = client.Get(config.Elasticsearch.Url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"error":  "",
		"data":   map[string]interface{}{},
	})
}

type Alert struct {
	Name          string `json:"name"`
	Key           string `json:"key"`
	HostID        string `json:"host_id"`
	Elasticsearch string `json:"elasticsearch"`
	Index         string `json:"index"`
	QueryString   string `json:"query_string"`
	Delay         string `json:"delay"`
	Threshold     string `json:"threshold"`
}

type CreatAlertParamBody struct {
	HostName  string `json:"hostname" binding:"required" example:"Zabbix server"`
	Delay     string `json:"delay" binding:"required" example:"3m"`
	Threshold string `json:"threshold" binding:"required" example:">=10"`
}

type CreatAlertParamQuery struct {
	Name        string `form:"name" binding:"required"`
	Index       string `form:"index" binding:"required"`
	QueryString string `form:"query_string" binding:"required"`
}

// CreatAlert
// @Summary Creat Alert
// @Schemes http
// @Description 创建告警规则
// @Tags alert
// @Accept json
// @Produce json
// @Param name query string true "名称"
// @Param index query string true "索引"
// @Param query_string query string true "查询字符串"
// @Param request body CreatAlertParamBody true "默认配置"
// @Success 200 {string} Success
// @Security BasicAuth
// @Router /alert/creat [post]
func CreatAlert(c *gin.Context) {

	var body CreatAlertParamBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}
	var query CreatAlertParamQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}

	config := c.MustGet("config").(configs.Config)
	username := config.Elasticsearch.Username
	password := config.Elasticsearch.Password
	elasticsearch := config.Elasticsearch.Url

	name := query.Name
	hash := md5.Sum([]byte(name))
	key := hex.EncodeToString(hash[:])
	hostName := body.HostName
	delay := body.Delay
	threshold := body.Threshold
	index := query.Index
	queryString := query.QueryString
	posts := connector.GeneratePosts(queryString, delay)
	url := fmt.Sprintf("%s/%s/_search", elasticsearch, index)

	zabbix := connector.NewZabbix(config.Zabbix.Url, config.Zabbix.Token)
	host, err := zabbix.GetHostByName(hostName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}
	itemID, err := zabbix.CreateItem(name, key, host.HostID, delay, username, password, url, posts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}
	TriggerID, err := zabbix.CreateTrigger(hostName, name, key, threshold)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"error":  "",
		"data": map[string]interface{}{
			"itemID":    itemID,
			"TriggerID": TriggerID,
		},
	})
}

type DeleteAlertParamQuery struct {
	Name string `form:"name" binding:"required"`
}

// DeleteAlert
// @Summary Delete Alert
// @Schemes http
// @Description 删除告警规则
// @Tags alert
// @Accept json
// @Produce json
// @Param name query string true "名称"
// @Success 204 {string} Success
// @Security BasicAuth
// @Router /alert/delete [delete]
func DeleteAlert(c *gin.Context) {
	var query DeleteAlertParamQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}

	itemName := query.Name
	config := c.MustGet("config").(configs.Config)
	zabbix := connector.NewZabbix(config.Zabbix.Url, config.Zabbix.Token)

	item, err := zabbix.GetItemByName(itemName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}
	_, err = zabbix.DeleteItemByID(item.ItemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "failure",
			"error":  err.Error(),
			"data":   map[string]interface{}{},
		})
		return
	}

	c.JSON(http.StatusNoContent, gin.H{
		"status": "success",
		"error":  "",
		"data": map[string]interface{}{
			"itemName": itemName,
		},
	})
}

// GetAlert
// @Summary Get Alerts
// @Schemes http
// @Description 查看所有告警规则
// @Tags alert
// @Accept json
// @Produce json
// @Success 200 {string} Success
// @Security BasicAuth
// @Router /alert/get [get]
func GetAlert(c *gin.Context) {
	config := c.MustGet("config").(configs.Config)
	zabbix := connector.NewZabbix(config.Zabbix.Url, config.Zabbix.Token)
	items, _ := zabbix.GetItems()

	var alerts []Alert
	for i := range items {
		trigger, _ := zabbix.GetTriggerByName(items[i].Name)
		alert := Alert{
			Name:          items[i].Name,
			Key:           items[i].Key,
			HostID:        items[i].HostID,
			Elasticsearch: items[i].GetElasticsearch(),
			Index:         items[i].GetIndex(),
			QueryString:   items[i].GetQueryString(),
			Delay:         items[i].Delay,
			Threshold:     trigger.GetThreshold(),
		}
		alerts = append(alerts, alert)
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"error":  "",
		"data":   alerts,
	})
}

func main() {
	r := gin.Default()

	// 加载配置文件，获取配置对象
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	config, err := configs.LoadConfig(fmt.Sprintf("%s/configs/config.yaml", dir))
	if err != nil {
		panic(err)
	}

	// 将配置对象存储在 Gin 上下文中
	r.Use(func(c *gin.Context) {
		c.Set("config", config)
		c.Next()
	})

	// Basic Authentication middleware
	authorized := r.Group("/api/v1", gin.BasicAuth(gin.Accounts{
		config.BasicAuth.Username: config.BasicAuth.Password,
	}))

	// 路由和处理程序
	docs.SwaggerInfo.BasePath = "/api/v1"
	v1 := authorized
	{
		ag := v1.Group("/alert")
		{
			ag.POST("/creat", CreatAlert)
			ag.GET("/get", GetAlert)
			ag.DELETE("/delete", DeleteAlert)
		}
	}
	r.GET("/api/v1/monitor/health_check", HealthCheck)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 启动服务器
	addr := fmt.Sprintf("%s:%s", config.Server.Addr, config.Server.Port)
	err = r.Run(addr)
	if err != nil {
		panic(err)
	}
}
