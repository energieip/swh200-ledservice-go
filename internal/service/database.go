package service

import "github.com/energieip/common-led-go/pkg/driverled"

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
		ledStored, err := s.db.GetRecord(driverled.DbStatus, driverled.TableName, criteria)
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
		dbID, err = s.db.InsertRecord(driverled.DbStatus, driverled.TableName, s.leds[led.Mac])
	} else {
		err = s.db.UpdateRecord(driverled.DbStatus, driverled.TableName, dbID, s.leds[led.Mac])
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
	ledStored, err := s.db.GetRecord(driverled.DbStatus, driverled.TableName, criteria)
	if err != nil || ledStored == nil {
		return nil
	}
	light, _ := driverled.ToLed(ledStored)
	return light
}
