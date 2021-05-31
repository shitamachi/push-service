package config_entries

type ApplePushSecretConfigItem struct {
	BundleID string `json:"bundle_id"`
	AuthKey  string `json:"auth_key"`
	KeyID    string `json:"key_id"`
	TeamID   string `json:"team_id"`
}

type ApplePushSecretConfig struct {
	Items map[string]ApplePushSecretConfigItem `json:"items"`
}

type FirebaseConfigItem struct {
	PackageName               string `json:"package_name"`
	ServiceAccountFileContent string `json:"service_account_file_content"`
}

type FirebaseConfig struct {
	Items map[string]FirebaseConfigItem `json:"items"`
}
