package command

import (
	"github.com/mattermost/mattermost-server/v5/model"

	"github.com/standup-raven/standup-raven/server/config"
)

func commandConfig() *Config {
	return &Config{
		AutocompleteData: &model.AutocompleteData{
			Trigger:  "config",
			HelpText: "Open channel standup configuration dialog.",
			RoleID:   model.SYSTEM_USER_ROLE_ID,
		},
		ExtraHelpText: "",
		Validate:      validateCommandConfig,
		Execute:       executeCommandConfig,
	}
}

func validateCommandConfig(args []string, context Context) (*model.CommandResponse, *model.AppError) {
	return nil, nil
}

func executeCommandConfig(args []string, context Context) (*model.CommandResponse, *model.AppError) {
	config.Mattermost.PublishWebSocketEvent(
		"open_config_modal",
		map[string]interface{}{
			"channel_id": context.CommandArgs.ChannelId,
		},
		&model.WebsocketBroadcast{
			UserId: context.CommandArgs.UserId,
		},
	)

	return &model.CommandResponse{
		ResponseType: model.COMMAND_RESPONSE_TYPE_EPHEMERAL,
		Text:         "Configure your standup in the open modal!", // TODO: update this message to something more elegant
	}, nil
}
