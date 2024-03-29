package utils

import (
	"os/user"

	"github.com/luyomo/OhMyTiUP/pkg/logger/log"
)

// CurrentUser returns current login user
func CurrentUser() string {
	user, err := user.Current()
	if err != nil {
		log.Errorf("Get current user: %s", err)
		return "root"
	}
	return user.Username
}

// UserHome returns home directory of current user
func UserHome() string {
	user, err := user.Current()
	if err != nil {
		log.Errorf("Get current user home: %s", err)
		return "root"
	}
	return user.HomeDir
}
