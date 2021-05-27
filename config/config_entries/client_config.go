package config_entries

type PushType = string

const (
	ApplePush    PushType = "apple"
	FirebasePush PushType = "firebase"
)

type ClientConfigItem struct {
	PushType PushType `json:"push_type"`
}
