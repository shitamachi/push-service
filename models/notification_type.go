package models

import "strconv"

type NotificationType int

func (n *NotificationType) String() string {
	return strconv.Itoa(int(*n))
}
