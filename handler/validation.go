package handler

import (
	"encoding/json"
	"fmt"
	"github.com/shitamachi/push-service/config"
	"github.com/shitamachi/push-service/config/config_entries"
	"github.com/shitamachi/push-service/models"
	"github.com/sideshow/apns2"
)

func ValidateReqMessage(message, appId string) error {
	if len(message) <= 0 {
		return fmt.Errorf("message content is empty")
	}
	clientConf, ok := config.GlobalConfig.ClientConfig[appId]
	if !ok {
		return fmt.Errorf("request app ID (%s) cannot match any of the items in the configuration file", appId)
	}
	switch clientConf.PushType {
	case config_entries.ApplePush:
		err := json.Unmarshal([]byte(message), &apns2.Notification{})
		if err != nil {
			return err
		}
	case config_entries.FirebasePush:
		err := json.Unmarshal([]byte(message), &models.FirebaseMessage{})
		if err != nil {
			return err
		}
	}
	return nil
}
