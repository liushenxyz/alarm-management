package configs

import (
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	BasicAuth     BasicAuthConfig     `yaml:"basic"`
	Zabbix        ZabbixConfig        `yaml:"zabbix"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
	Port string `yaml:"port"`
}

type BasicAuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ZabbixConfig struct {
	Url   string `yaml:"url"`
	Token string `yaml:"token"`
}

type ElasticsearchConfig struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func LoadConfig(file string) (Config, error) {
	var config Config

	data, err := os.ReadFile(file)
	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
