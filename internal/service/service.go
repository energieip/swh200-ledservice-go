package service

import (
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

//Initialize service
func (s *LedService) Initialize(confFile string) error {
	s.leds = make(map[string]*driverled.Led)
	hostname, _ := os.Hostname()
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
	err = s.db.CreateDB(driverled.DbStatus)
	if err != nil {
		rlog.Warn("Create DB ", err.Error())
	}
	err = s.db.CreateTable(driverled.DbStatus, driverled.TableName, &driverled.Led{})
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
		IP:               conf.LocalBroker.IP,
		Port:             conf.LocalBroker.Port,
		ClientName:       clientID,
		Callbacks:        callbacks,
		LogLevel:         conf.LogLevel,
		User:             conf.LocalBroker.Login,
		Password:         conf.LocalBroker.Password,
		ClientKey:        conf.LocalBroker.KeyPath,
		ServerCertificat: conf.LocalBroker.CaPath,
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
