package models

import "strconv"

type NotificationType int

const (
	UnknownPushType NotificationType = iota
	OpenBook
	OpenHtmlPage
	OpenGiftPack
	OpenRewardTaskPage
)

func (n *NotificationType) String() string {
	return strconv.Itoa(int(*n))
}
