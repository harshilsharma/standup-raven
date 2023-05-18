package utils

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"

	"github.com/standup-raven/standup-raven/server/config"
	"github.com/standup-raven/standup-raven/server/logger"
)

// UserIcon generates the URL to user's profile picture
func UserIcon(userID string) string {
	return fmt.Sprintf("![User Avatar]("+config.UserIconURL+" "+config.UserIconSize+")", userID)
}

// GetUserRoles fetches all roles of a user - system level, team level and channel level
func GetUserRoles(userID, channelID string) ([]string, *model.AppError) {
	var rolesString string

	channelRoles, appErr := getUserChannelRoles(userID, channelID)
	if appErr != nil {
		return nil, appErr
	}
	rolesString += channelRoles

	channel, appErr := config.Mattermost.GetChannel(channelID)
	if appErr != nil {
		return nil, appErr
	}

	teamRoles, appErr := getUserTeamRoles(userID, channel.TeamId)
	if appErr != nil {
		return nil, appErr
	}

	rolesString += " " + teamRoles

	systemRoles, appErr := getUserSystemRoles(userID)
	if appErr != nil {
		return nil, appErr
	}

	rolesString += " " + systemRoles

	return strings.Split(rolesString, " "), nil
}

func getUserChannelRoles(userID, channelID string) (string, *model.AppError) {
	channelMember, appErr := config.Mattermost.GetChannelMember(channelID, userID)
	if appErr != nil {
		logger.Error(appErr.Error(), appErr, nil)
		return "", appErr
	}
	return channelMember.Roles, nil
}

func getUserTeamRoles(userID, teamID string) (string, *model.AppError) {
	teamMember, appErr := config.Mattermost.GetTeamMember(teamID, userID)
	if appErr != nil {
		logger.Error(appErr.Error(), appErr, nil)
		return "", appErr
	}
	return teamMember.Roles, nil
}

func getUserSystemRoles(userID string) (string, *model.AppError) {
	user, appErr := config.Mattermost.GetUser(userID)
	if appErr != nil {
		logger.Error(appErr.Error(), appErr, nil)
		return "", appErr
	}

	return user.Roles, nil
}
