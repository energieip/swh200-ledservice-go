package service

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/energieip/common-database-go/pkg/database"
	"github.com/energieip/common-led-go/pkg/driverled"
	"github.com/energieip/common-network-go/pkg/network"
	"github.com/energieip/swh200-ledservice-go/pkg/config"
	"github.com/energieip/swh200-ledservice-go/pkg/tools"
	"github.com/romana/rlog"
)

//LedService content
type LedService struct {
	db     database.DatabaseInterface
	broker network.NetworkInterface //Local Broker for drivers communication
	leds   map[string]*driverled.Led
	mac    string //Switch mac address
}

func (s *LedService) updateDatabase(led driverled.Led) error {
	if val, ok := s.leds[led.Mac]; ok {
		led.ID = val.ID
		if *val == led {
			// No change to register
			return nil
		}
		s.leds[led.Mac] = &led
		return s.db.UpdateRecord(driverled.DbName, driverled.TableName, led.ID, s.leds[led.Mac].ToMapInterface())
	}

	var dbID string
	s.leds[led.Mac] = &led
	criteria := make(map[string]interface{})
	criteria["mac"] = led.Mac
	criteria["switchMac"] = led.SwitchMac
	ledStored, err := s.db.GetRecord(driverled.DbName, driverled.TableName, criteria)
	if err != nil || ledStored == nil {
		// Check if the serial already exist in database (case restart process)
		dbID, err = s.db.InsertRecord(driverled.DbName, driverled.TableName, s.leds[led.Mac].ToMapInterface())
		if err != nil {
			return err
		}

	} else {
		m := ledStored.(map[string]interface{})
		id, ok := m["id"]
		if !ok {
			id, ok = m["ID"]
		}
		if ok {
			dbID = id.(string)
			s.leds[led.Mac] = &led
			err = s.db.UpdateRecord(driverled.DbName, driverled.TableName, dbID, s.leds[led.Mac].ToMapInterface())
			if err != nil {
				return err
			}
		}
	}
	s.leds[led.Mac].ID = dbID
	return nil
}

func (s *LedService) onSetup(client network.Client, msg network.Message) {
	rlog.Debug("LED service onSetup: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()[:]))
	var led driverled.LedSetup
	err := json.Unmarshal(msg.Payload(), &led)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	val, ok := s.leds[led.Mac]
	if !ok {
		rlog.Warn("Cannot send find driver " + led.Mac)
		return
	}
	url := "/write/" + val.Topic + "/" + driverled.UrlSetup
	ledDump, _ := led.ToJSON()
	err = s.broker.SendCommand(url, ledDump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration for driver " + led.Mac + " err: " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + led.Mac)
	}
}

func (s *LedService) onUpdate(client network.Client, msg network.Message) {
	rlog.Debug("LED service update settings: Received topic: " + msg.Topic() + " payload: " + string(msg.Payload()[:]))
	var conf driverled.LedConf
	err := json.Unmarshal(msg.Payload(), &conf)
	if err != nil {
		rlog.Error("Error during parsing", err.Error())
		return
	}
	var topic string
	if val, ok := s.leds[conf.Mac]; ok {
		topic = val.Topic
	} else {
		criteria := make(map[string]interface{})
		criteria["mac"] = conf.Mac
		criteria["switchMac"] = s.mac
		ledStored, err := s.db.GetRecord(driverled.DbName, driverled.TableName, criteria)
		if err != nil || ledStored == nil {
			return
		}
		l := ledStored.(map[string]interface{})
		if url, ok := l["topic"]; ok {
			topic = url.(string)
		}
	}
	if topic == "" {
		return
	}

	url := "/write/" + topic + "/update/settings"
	dump, _ := conf.ToJSON()
	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration to driver " + conf.Mac + " err " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + conf.Mac)
	}
}

func (s *LedService) onDriverHello(client network.Client, msg network.Message) {
	rlog.Debug("LED service: Received hello topic: " + msg.Topic() + " payload: " + string(msg.Payload()[:]))
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
	rlog.Debug("LED service driver status: Received topic: " + topic + " payload: " + string(msg.Payload()[:]))
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

//Initialize service
func (s *LedService) Initialize(confFile string) error {
	s.leds = make(map[string]*driverled.Led)
	hostname, err := os.Hostname()
	if err != nil {
		rlog.Error("Cannot read hostname " + err.Error())
		return err
	}
	clientID := "LED" + hostname
	s.mac = strings.ToUpper(strings.Replace(tools.GetMac(), ":", "", -1))

	conf, err := config.ReadConfig(confFile)
	if err != nil {
		rlog.Error("Cannot parse configuration file " + err.Error())
		return err
	}
	os.Setenv("RLOG_LOG_LEVEL", *conf.LogLevel)
	os.Setenv("RLOG_LOG_NOTIME", "yes")
	rlog.UpdateEnv()
	rlog.Info("Starting LED service")

	db, err := database.NewDatabase(database.RETHINKDB)
	if err != nil {
		rlog.Error("database err " + err.Error())
		return err
	}

	confDb := database.DatabaseConfig{}
	confDb.IP = conf.DatabaseIP
	confDb.Port = conf.DatabasePort
	err = db.Initialize(confDb)
	if err != nil {
		rlog.Error("Cannot connect to database " + err.Error())
		return err
	}
	s.db = db
	err = s.db.CreateDB(driverled.DbName)
	if err != nil {
		rlog.Warn("Create DB ", err.Error())
	}
	err = s.db.CreateTable(driverled.DbName, driverled.TableName)
	if err != nil {
		rlog.Warn("Create table ", err.Error())
	}

	driversBroker, err := network.NewNetwork(network.MQTT)
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.DriversBrokerIP + " error: " + err.Error())
		return err
	}
	s.broker = driversBroker

	callbacks := make(map[string]func(client network.Client, msg network.Message))
	callbacks["/read/led/+/"+driverled.UrlHello] = s.onDriverHello
	callbacks["/read/led/+/"+driverled.UrlStatus] = s.onDriverStatus
	callbacks["/write/switch/"+s.mac+"/led/setup/config"] = s.onSetup
	callbacks["/write/switch/"+s.mac+"/led/update/settings"] = s.onUpdate

	confDrivers := network.NetworkConfig{}
	confDrivers.IP = conf.DriversBrokerIP
	confDrivers.Port = conf.DriversBrokerPort
	confDrivers.ClientName = clientID
	confDrivers.Callbacks = callbacks
	confDrivers.LogLevel = *conf.LogLevel
	err = driversBroker.Initialize(confDrivers)
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.DriversBrokerIP + " error: " + err.Error())
		return err
	}

	rlog.Info(clientID + " connected to drivers broker " + conf.DriversBrokerIP)

	rlog.Info("LED service started")
	return nil
}

//Stop service
func (s *LedService) Stop() {
	rlog.Info("Stopping LED service")
	s.broker.Disconnect()
	s.db.Close()
	rlog.Info("LED service stopped")
}

//Run service mainloop
func (s *LedService) Run() error {
	select {}
}
