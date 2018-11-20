package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

//Configuration configuration structure
type Configuration struct {
	DatabaseIP        string  `json:"database_ip"`
	DatabasePort      string  `json:"database_port"`
	DriversBrokerIP   string  `json:"drivers_broker_ip"`
	DriversBrokerPort string  `json:"drivers_broker_port"`
	LogLevel          *string `json:"log_level"`
}

//ReadConfig parse the configuration file
func ReadConfig(path string) (*Configuration, error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	var config Configuration

	json.Unmarshal(byteValue, &config)
	if config.LogLevel == nil {
		level := "INFO"
		config.LogLevel = &level
	}
	return &config, nil
}
