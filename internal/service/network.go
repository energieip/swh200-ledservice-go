package service

import (
	"encoding/json"
	"strings"

	"github.com/energieip/common-led-go/pkg/driverled"
	"github.com/energieip/common-network-go/pkg/network"
	"github.com/romana/rlog"
)

//SetupCmd setup command
type SetupCmd struct {
	driverled.LedSetup
	CmdType string `json:"cmdType"`
}

// ToJSON dump SetupCmd struct
func (led SetupCmd) ToJSON() (string, error) {
	inrec, err := json.Marshal(led)
	if err != nil {
		return "", err
	}
	return string(inrec[:]), err
}

//UpdateCmd update command
type UpdateCmd struct {
	driverled.LedConf
	CmdType string `json:"cmdType"`
}

// ToJSON dump UpdateCmd struct
func (led UpdateCmd) ToJSON() (string, error) {
	inrec, err := json.Marshal(led)
	if err != nil {
		return "", err
	}
	return string(inrec[:]), err
}

func (s *LedService) onSetup(client network.Client, msg network.Message) {
	rlog.Debug("LED service onSetup: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var led driverled.LedSetup
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	topic := s.getTopic(led.Mac)
	if topic == "" {
		rlog.Warn("Cannot find driver " + led.Mac)
		return
	}
	url := "/write/" + topic + "/" + driverled.UrlSetup

	setupCmd := SetupCmd{}
	setupCmd.LedSetup = led
	setupCmd.CmdType = "setup"
	dump, _ := setupCmd.ToJSON()

	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration for driver " + led.Mac + " err: " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + led.Mac + " on topic: " + url + " dump: " + dump)
	}
}

func (s *LedService) onUpdate(client network.Client, msg network.Message) {
	rlog.Debug("LED service update settings: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var conf driverled.LedConf
	err := json.Unmarshal(msg.Payload(), &conf)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	topic := s.getTopic(conf.Mac)
	if topic == "" {
		rlog.Warn("Cannot find driver " + conf.Mac)
		return
	}

	url := "/write/" + topic + "/update/settings"

	setupCmd := UpdateCmd{}
	setupCmd.LedConf = conf
	setupCmd.CmdType = "update"
	dump, _ := setupCmd.ToJSON()

	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration to driver " + conf.Mac + " err " + err.Error())
	} else {
		rlog.Info("New update has been sent to " + conf.Mac + " on topic: " + url + " dump: " + dump)
	}
}

func (s *LedService) onDriverHello(client network.Client, msg network.Message) {
	rlog.Debug("LED service: Received hello topic: " + msg.Topic() + " payload: " + string(msg.Payload()))
	var led driverled.Led
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}

	led.IsConfigured = false
	led.Protocol = "MQTT"
	led.SwitchMac = s.mac
	err = s.updateDatabase(led)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
		return
	}
	rlog.Infof("New LED driver %v stored on database ", led.Mac)
}

func (s *LedService) onDriverStatus(client network.Client, msg network.Message) {
	topic := msg.Topic()
	rlog.Debug("LED service driver status: Received topic: " + topic + " payload: " + string(msg.Payload()))
	var led driverled.Led
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	led.SwitchMac = s.mac
	led.Protocol = "MQTT"
	topics := strings.Split(topic, "/")
	led.Topic = topics[2] + "/" + topics[3]
	err = s.updateDatabase(led)
	if err != nil {
		rlog.Error("Error during database update ", err.Error())
	}
}
