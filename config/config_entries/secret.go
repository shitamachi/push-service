package config_entries

type ApplePushSecretConfig struct {
	BundleID string `json:"bundle_id"`
	AuthKey  string `json:"auth_key"`
	KeyID    string `json:"key_id"`
	TeamID   string `json:"team_id"`
}

type FirebaseConfig struct {
	PackageName               string `json:"package_name"`
	ServiceAccountFileContent string `json:"service_account_file_content"`
}
