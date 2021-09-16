package models

type PlatformTokenType = int

const (
	UnknownPlatform PlatformTokenType = iota
	FcmToken
	AppleDeviceToken
)
