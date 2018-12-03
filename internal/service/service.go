package service

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/energieip/common-database-go/pkg/database"
	"github.com/energieip/common-led-go/pkg/driverled"
	"github.com/energieip/common-network-go/pkg/network"
	pkg "github.com/energieip/common-service-go/pkg/service"
	"github.com/energieip/common-tools-go/pkg/tools"
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
	var dbID string
	if val, ok := s.leds[led.Mac]; ok {
		led.ID = val.ID
		dbID = val.ID
		if *val == led {
			// No change to register
			return nil
		}
	}

	s.leds[led.Mac] = &led
	if dbID == "" {
		// Check if the serial already exist in database (case restart process)
		criteria := make(map[string]interface{})
		criteria["Mac"] = led.Mac
		criteria["SwitchMac"] = s.mac
		ledStored, err := s.db.GetRecord(driverled.DbName, driverled.TableName, criteria)
		if err == nil && ledStored != nil {
			m := ledStored.(map[string]interface{})
			id, ok := m["id"]
			if !ok {
				id, ok = m["ID"]
			}
			if ok {
				dbID = id.(string)
			}
		}
	}
	var err error

	if dbID == "" {
		dbID, err = s.db.InsertRecord(driverled.DbName, driverled.TableName, s.leds[led.Mac])
	} else {
		err = s.db.UpdateRecord(driverled.DbName, driverled.TableName, dbID, s.leds[led.Mac])
	}
	if err != nil {
		return err
	}
	s.leds[led.Mac].ID = dbID
	return nil
}

func (s *LedService) getLed(mac string) *driverled.Led {
	if val, ok := s.leds[mac]; ok {
		return val
	}
	criteria := make(map[string]interface{})
	criteria["Mac"] = mac
	criteria["SwitchMac"] = s.mac
	ledStored, err := s.db.GetRecord(driverled.DbName, driverled.TableName, criteria)
	if err != nil || ledStored == nil {
		return nil
	}
	light, _ := driverled.ToLed(ledStored)
	return light
}

func (s *LedService) getTopic(mac string) string {
	light := s.getLed(mac)
	if light != nil {
		return light.Topic
	}
	return ""
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
	ledDump, _ := led.ToJSON()
	err = s.broker.SendCommand(url, ledDump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration for driver " + led.Mac + " err: " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + led.Mac + " on topic: " + url)
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
	dump, _ := conf.ToJSON()
	err = s.broker.SendCommand(url, dump)
	if err != nil {
		rlog.Errorf("Cannot send new configuration to driver " + conf.Mac + " err " + err.Error())
	} else {
		rlog.Info("New configuration has been sent to " + conf.Mac + " on topic: " + topic)
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

	conf, err := pkg.ReadServiceConfig(confFile)
	if err != nil {
		rlog.Error("Cannot parse configuration file " + err.Error())
		return err
	}
	os.Setenv("RLOG_LOG_LEVEL", conf.LogLevel)
	os.Setenv("RLOG_LOG_NOTIME", "yes")
	rlog.UpdateEnv()
	rlog.Info("Starting LED service")

	db, err := database.NewDatabase(database.RETHINKDB)
	if err != nil {
		rlog.Error("database err " + err.Error())
		return err
	}

	confDb := database.DatabaseConfig{
		IP:   conf.DB.ClientIP,
		Port: conf.DB.ClientPort,
	}
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
	err = s.db.CreateTable(driverled.DbName, driverled.TableName, &driverled.Led{})
	if err != nil {
		rlog.Warn("Create table ", err.Error())
	}

	driversBroker, err := network.NewNetwork(network.MQTT)
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}
	s.broker = driversBroker

	callbacks := make(map[string]func(client network.Client, msg network.Message))
	callbacks["/read/led/+/"+driverled.UrlHello] = s.onDriverHello
	callbacks["/read/led/+/"+driverled.UrlStatus] = s.onDriverStatus
	callbacks["/write/switch/led/setup/config"] = s.onSetup
	callbacks["/write/switch/led/update/settings"] = s.onUpdate

	confDrivers := network.NetworkConfig{
		IP:         conf.LocalBroker.IP,
		Port:       conf.LocalBroker.Port,
		ClientName: clientID,
		Callbacks:  callbacks,
		LogLevel:   conf.LogLevel,
	}
	err = driversBroker.Initialize(confDrivers)
	if err != nil {
		rlog.Error("Cannot connect to broker " + conf.LocalBroker.IP + " error: " + err.Error())
		return err
	}

	rlog.Info(clientID + " connected to drivers broker " + conf.LocalBroker.IP)
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
