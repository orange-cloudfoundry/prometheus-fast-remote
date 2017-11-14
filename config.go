// Copyright 2017 Orange
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
)

func checkOverflow(m map[string]interface{}, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("%s: unknown fields: %s", ctx, strings.Join(keys, ", "))
	}
	return nil
}
func Load(s string) (*Config, error) {
	cfg := &Config{}
	err := yaml.Unmarshal([]byte(s), cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
func LoadFile(filename string) (*Config, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg, err := Load(string(content))
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

type Config struct {
	KairosUrl    string                 `yaml:"kairos_url"`
	SkipInsecure bool                   `yaml:"skip_insecure"`
	ListenAddr   string                 `yaml:"listen_addr"`
	LogLevel     string                 `yaml:"log_level"`
	LogJson      bool                   `yaml:"log_json"`
	NoColor      bool                   `yaml:"no_color"`
	Workers      int                    `yaml:"workers"`
	XXX          map[string]interface{} `yaml:",inline" json:"-"`
}

func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	if c.KairosUrl == "" {
		return fmt.Errorf("Config: kairos_url must be set")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if c.ListenAddr == "" {
		c.ListenAddr = "0.0.0.0:" + port
	}
	if len(strings.Split(c.ListenAddr, ":")) < 2 {
		c.ListenAddr += ":" + port
	}
	if c.Workers <= 0 {
		c.Workers = 5
	}
	err := checkOverflow(c.XXX, "Config")
	if err != nil {
		return err
	}
	c.loadLogConfig()
	return nil
}
func (c Config) loadLogConfig() {

	if c.LogJson {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			DisableColors: c.NoColor,
		})
	}
	if c.LogLevel == "" {
		return
	}
	switch strings.ToUpper(c.LogLevel) {
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
		return
	case "WARN":
		log.SetLevel(log.WarnLevel)
		return
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
		return
	case "PANIC":
		log.SetLevel(log.PanicLevel)
		return
	case "FATAL":
		log.SetLevel(log.FatalLevel)
		return
	}
}
