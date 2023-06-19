package notification

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"

	"github.com/standup-raven/standup-raven/server/config"
	"github.com/standup-raven/standup-raven/server/logger"
	"github.com/standup-raven/standup-raven/server/otime"
	"github.com/standup-raven/standup-raven/server/standup"
	"github.com/standup-raven/standup-raven/server/util"
)

type ChannelNotificationStatus struct {
	WindowOpenNotificationSent  bool `json:"windowOpenNotificationSent"`
	WindowCloseNotificationSent bool `json:"windowCloseNotificationSent"`
	StandupReportSent           bool `json:"standupReportSent"`
}

const (
	// statuses for standup notification in a channel
	ChannelNotificationStatusSent   = "sent"
	ChannelNotificationStatusNotYet = "not_yet"
	ChannelNotificationStatusSend   = "send"

	// standup report visibilities
	ReportVisibilityPublic  = "public"
	ReportVisibilityPrivate = "private"
)

// SendNotificationsAndReports checks for all standup channels and sends
// notifications and standup reports as needed.
// This is the entry point of the whole standup cycle.
func SendNotificationsAndReports() error {
	channelIDs, err := standup.GetStandupChannels()
	if err != nil {
		return err
	}

	pendingWindowOpenNotificationChannelIDs,
		pendingWindowCloseNotificationChannelIDs,
		pendingStandupReportChannelIDs,
		err := filterChannelNotification(channelIDs)

	if err != nil {
		return err
	}

	sendWindowOpenNotification(pendingWindowOpenNotificationChannelIDs)
	if err := sendWindowCloseNotification(pendingWindowCloseNotificationChannelIDs); err != nil {
		return err
	}
	if err := sendAllStandupReport(pendingStandupReportChannelIDs); err != nil {
		return err
	}

	return nil
}

func sendAllStandupReport(channelIDs []string) error {
	for _, channelID := range channelIDs {
		standupConfig, err := standup.GetStandupConfig(channelID)
		if err != nil {
			return err
		}
		if standupConfig == nil {
			return errors.New("standup not configured for channel: " + channelID)
		}
		standupReportError := SendStandupReport([]string{channelID}, otime.Now(standupConfig.Timezone), ReportVisibilityPublic, "", true)
		if standupReportError != nil {
			return standupReportError
		}
	}
	return nil
}

// GetNotificationStatus gets the notification status for specified channel
func GetNotificationStatus(channelID string) (*ChannelNotificationStatus, error) {
	logger.Debug(fmt.Sprintf("Fetching notification status for channel: %s", channelID), nil)
	standupConfig, err := standup.GetStandupConfig(channelID)
	if err != nil {
		return nil, err
	}
	if standupConfig == nil {
		return nil, errors.New("standup not configured for channel: " + channelID)
	}
	key := fmt.Sprintf("%s_%s_%s", config.CacheKeyPrefixNotificationStatus, channelID, util.GetCurrentDateString(standupConfig.Timezone))
	data, appErr := config.Mattermost.KVGet(util.GetKeyHash(key))
	if appErr != nil {
		logger.Error("Couldn't get notification status from KV store", appErr, nil)
		return nil, errors.New(appErr.Error())
	} else if len(data) == 0 {
		return &ChannelNotificationStatus{}, nil
	}

	status := &ChannelNotificationStatus{}
	if err := json.Unmarshal(data, status); err != nil {
		logger.Error("Couldn't unmarshal notification status data into struct", err, map[string]interface{}{"channelID": channelID, "data": string(data)})
		return nil, err
	}

	logger.Debug(fmt.Sprintf("notification status for channel: %s, %v", channelID, status), nil)
	return status, nil
}

// SendStandupReport sends standup report for all channel IDs specified
func SendStandupReport(channelIDs []string, date otime.OTime, visibility string, userID string, updateStatus bool) error {
	for _, channelID := range channelIDs {
		logger.Info("Sending standup report for channel: "+channelID+" time: "+date.GetDateString(), nil)

		standupConfig, err := standup.GetStandupConfig(channelID)
		if err != nil {
			return err
		}

		if standupConfig == nil {
			return errors.New("standup not configured for channel: " + channelID)
		}

		// standup of all channel standup members
		var members []*standup.UserStandup

		// names of channel standup members who haven't yet submitted their standup
		var membersNoStandup []string
		for _, userID := range standupConfig.Members {
			userStandup, err := standup.GetUserStandup(userID, channelID, date)
			if err != nil {
				return err
			} else if userStandup == nil {
				// if user has not submitted standup
				logger.Info("Could not fetch standup for user: "+userID, nil)

				user, appErr := config.Mattermost.GetUser(userID)
				if appErr != nil {
					logger.Error("Couldn't fetch user", appErr, map[string]interface{}{"userID": userID})
					return errors.New(appErr.Error())
				}

				membersNoStandup = append(membersNoStandup, user.Username)

				continue
			}

			members = append(members, userStandup)
		}

		members, err = sortUserStandups(members)
		if err != nil {
			return err
		}

		post, err := generateReport(
			standupConfig,
			members,
			membersNoStandup,
			channelID,
			date,
		)

		if err != nil {
			return err
		}

		if visibility == ReportVisibilityPrivate {
			config.Mattermost.SendEphemeralPost(userID, post)
		} else {
			_, appErr := config.Mattermost.CreatePost(post)
			if appErr != nil {
				logger.Error("Couldn't create standup report post", appErr, nil)
				return errors.New(appErr.Error())
			}
		}

		if err := deleteReminderPosts(channelID); err != nil {
			// log and continue. This shouldn't affect primary flow
			logger.Error("Error occurred while deleting reminder posts for channel: "+channelID, err, nil)
		}

		if updateStatus {
			notificationStatus, err := GetNotificationStatus(channelID)
			if err != nil {
				continue
			}

			notificationStatus.StandupReportSent = true
			if err := SetNotificationStatus(channelID, notificationStatus); err != nil {
				return err
			}
		}
	}

	return nil
}

func generateReport(
	standupConfig *standup.Config,
	members []*standup.UserStandup,
	membersNoStandup []string,
	channelID string,
	date otime.OTime,
) (*model.Post, error) {
	var post *model.Post
	var err error

	switch standupConfig.ReportFormat {
	case config.ReportFormatTypeAggregated:
		post, err = generateTypeAggregatedStandupReport(standupConfig, members, membersNoStandup, channelID, date)
	case config.ReportFormatUserAggregated:
		post, err = generateUserAggregatedStandupReport(standupConfig, members, membersNoStandup, channelID, date)
	default:
		err = errors.New("Unknown report format encountered for channel: " + channelID + ", report format: " + standupConfig.ReportFormat)
		logger.Error("Unknown report format encountered for channel", err, nil)
	}

	if err != nil {
		return nil, err
	}

	return post, err
}

func sortUserStandups(userStandups []*standup.UserStandup) ([]*standup.UserStandup, error) {
	// sorts user standups alphabetically by user's display name

	// get all user display names
	userStandupMapping := make(map[string]*standup.UserStandup, len(userStandups))

	// extract keys, which are the user display names
	keys := make([]string, 0)

	for _, userStandup := range userStandups {
		userDisplayName, err := getUserDisplayName(userStandup.UserID)
		if err != nil {
			return nil, err
		}

		userStandupMapping[userDisplayName] = userStandup
		keys = append(keys, userDisplayName)
	}

	// case insensitive sort of user display names
	sort.SliceStable(keys, func(i, j int) bool {
		return strings.ToLower(keys[i]) < strings.ToLower(keys[j])
	})

	// iterate the sorted array and arrange user standups in that order
	sortedUserStandups := make([]*standup.UserStandup, 0)
	for _, userDisplayName := range keys {
		sortedUserStandups = append(sortedUserStandups, userStandupMapping[userDisplayName])
	}

	return sortedUserStandups, nil
}

// SetNotificationStatus sets provided notification status for the specified channel ID.
func SetNotificationStatus(channelID string, status *ChannelNotificationStatus) error {
	standupConfig, err := standup.GetStandupConfig(channelID)
	if err != nil {
		return err
	}
	if standupConfig == nil {
		return errors.New("standup not configured for channel: " + channelID)
	}
	key := fmt.Sprintf("%s_%s_%s", config.CacheKeyPrefixNotificationStatus, channelID, util.GetCurrentDateString(standupConfig.Timezone))
	serializedStatus, err := json.Marshal(status)
	if err != nil {
		logger.Error("Couldn't marshal standup status data", err, nil)
		return err
	}

	if appErr := config.Mattermost.KVSet(util.GetKeyHash(key), serializedStatus); appErr != nil {
		logger.Error("Couldn't save standup status data into KV store", appErr, nil)
		return errors.New(appErr.Error())
	}

	return nil
}

// filterChannelNotification filters all provided standup channels into three categories -
// 1. channels requiring window open notification
// 2. channels requiring window close notification
// 3. channels requiring standup report
func filterChannelNotification(channelIDs map[string]string) ([]string, []string, []string, error) {
	logger.Debug("Filtering channels for sending notifications", nil)
	logger.Debug(fmt.Sprintf("Channels to process: %d", len(channelIDs)), nil, nil)

	var windowOpenNotificationChannels, windowCloseNotificationChannels, standupReportChannels []string

	for channelID := range channelIDs {
		logger.Debug(fmt.Sprintf("Processing channel: %s", channelID), nil)

		standupConfig, err := standup.GetStandupConfig(channelID)
		if err != nil {
			logger.Error("B", err, nil)
			return nil, nil, nil, err
		}

		if standupConfig == nil {
			logger.Error("Unable to find standup config for channel", nil, map[string]interface{}{"channelID": channelID})
			continue
		}

		if !standupConfig.Enabled {
			continue
		}

		if !isStandupDay(standupConfig) {
			continue
		}

		notificationStatus, err := GetNotificationStatus(channelID)
		if err != nil {
			logger.Error("A", err, nil)
			return nil, nil, nil, err
		}

		// we check in opposite order of time and check for just one notification to send.
		// This prevents expired notifications from being sent in case some of
		// the notifications were missed in the past

		if status := shouldSendStandupReport(notificationStatus, standupConfig); status == ChannelNotificationStatusSend {
			logger.Debug(fmt.Sprintf("Channel [%s] needs standup report", channelID), nil)
			standupReportChannels = append(standupReportChannels, channelID)
		} else if status == ChannelNotificationStatusSent {
			// pass
		} else if shouldSendWindowCloseNotification(notificationStatus, standupConfig) == ChannelNotificationStatusSend {
			if standupConfig.WindowCloseReminderEnabled {
				logger.Debug(fmt.Sprintf("Channel [%s] needs window close notification", channelID), nil)
				windowCloseNotificationChannels = append(windowCloseNotificationChannels, channelID)
			}
		} else if status == ChannelNotificationStatusSent {
			// pass
		} else if shouldSendWindowOpenNotification(notificationStatus, standupConfig) == ChannelNotificationStatusSend {
			if standupConfig.WindowOpenReminderEnabled {
				logger.Debug(fmt.Sprintf("Channel [%s] needs window open notification", channelID), nil)
				windowOpenNotificationChannels = append(windowOpenNotificationChannels, channelID)
			}
		}
	}

	logger.Debug(fmt.Sprintf(
		"Notifications filtered: open: %d, close: %d, reports: %d",
		len(windowOpenNotificationChannels),
		len(windowCloseNotificationChannels),
		len(standupReportChannels),
	), nil)
	return windowOpenNotificationChannels, windowCloseNotificationChannels, standupReportChannels, nil
}

// shouldSendWindowOpenNotification checks if window open notification should
// be sent to the channel with specified notification status
func shouldSendWindowOpenNotification(notificationStatus *ChannelNotificationStatus, standupConfig *standup.Config) string {
	if notificationStatus.WindowOpenNotificationSent {
		return ChannelNotificationStatusSent
	}

	now := otime.Now(standupConfig.Timezone).GetTimeWithSeconds(standupConfig.Timezone)
	next := standupConfig.WindowOpenTime.GetTimeWithSeconds(standupConfig.Timezone).Time

	if now.After(next) {
		return ChannelNotificationStatusSend
	}

	return ChannelNotificationStatusNotYet
}

// shouldSendWindowCloseNotification checks if window close notification should
// be sent to the channel with specified notification status
func shouldSendWindowCloseNotification(notificationStatus *ChannelNotificationStatus, standupConfig *standup.Config) string {
	if notificationStatus.WindowCloseNotificationSent {
		return ChannelNotificationStatusSent
	}

	windowDuration := standupConfig.WindowCloseTime.GetTime(standupConfig.Timezone).Time.Sub(standupConfig.WindowOpenTime.GetTime(standupConfig.Timezone).Time)
	targetDurationSeconds := windowDuration.Seconds() * config.WindowCloseNotificationDurationPercentage
	targetDuration, _ := time.ParseDuration(fmt.Sprintf("%fs", targetDurationSeconds))

	// now we just need to check if current time is targetDuration seconds after window open time
	if otime.Now(standupConfig.Timezone).GetTimeWithSeconds(standupConfig.Timezone).After(standupConfig.WindowOpenTime.GetTimeWithSeconds(standupConfig.Timezone).Add(targetDuration)) {
		return ChannelNotificationStatusSend
	}

	return ChannelNotificationStatusNotYet
}

// shouldSendStandupReport checks if standup report should
// be sent to the channel with specified notification status
func shouldSendStandupReport(notificationStatus *ChannelNotificationStatus, standupConfig *standup.Config) string {
	if notificationStatus.StandupReportSent {
		return ChannelNotificationStatusSent
	} else if otime.Now(standupConfig.Timezone).GetTimeWithSeconds(standupConfig.Timezone).After(standupConfig.WindowCloseTime.GetTimeWithSeconds(standupConfig.Timezone).Time) {
		return ChannelNotificationStatusSend
	}

	return ChannelNotificationStatusNotYet
}

// sendWindowOpenNotification sends window open notification to the specified channels
func sendWindowOpenNotification(channelIDs []string) {
	for _, channelID := range channelIDs {
		post := &model.Post{
			ChannelId: channelID,
			UserId:    config.GetConfig().BotUserID,
			Type:      model.POST_DEFAULT,
			Message:   "Please start filling your standup!",
		}

		post, appErr := config.Mattermost.CreatePost(post)
		if appErr != nil {
			logger.Error("Error sending window open notification for channel", appErr, map[string]interface{}{"channelID": channelID})
			continue
		}

		err := addReminderPost(post.Id, channelID)
		if err != nil {
			logger.Error("Couldn't add standup reminder posts", err, nil)
			continue
		}

		notificationStatus, err := GetNotificationStatus(channelID)
		if err != nil {
			continue
		}

		notificationStatus.WindowOpenNotificationSent = true
		if err := SetNotificationStatus(channelID, notificationStatus); err != nil {
			continue
		}
	}
}

// sendWindowCloseNotification sends window close notification to the specified channels
func sendWindowCloseNotification(channelIDs []string) error {
	for _, channelID := range channelIDs {
		standupConfig, err := standup.GetStandupConfig(channelID)
		if err != nil {
			return err
		}

		if standupConfig == nil {
			logger.Error("Unable to find standup config for channel", nil, map[string]interface{}{"channelID": channelID})
			continue
		}

		logger.Debug("Fetching members with pending standup reports", nil)

		var usersPendingStandup []string
		for _, userID := range standupConfig.Members {
			userStandup, err := standup.GetUserStandup(userID, channelID, otime.Now(standupConfig.Timezone))
			if err != nil {
				return err
			}

			if userStandup == nil {
				user, err := config.Mattermost.GetUser(userID)
				if err != nil {
					logger.Error("Couldn't find user with user ID", err, map[string]interface{}{"userID": userID})
					return err
				}

				usersPendingStandup = append(usersPendingStandup, user.Username)
			}
		}

		// no need to send reminder if everyone has filled their standup
		if len(usersPendingStandup) == 0 {
			logger.Debug("Not sending window close notification. No pending standups found.", nil, nil)
			return nil
		}

		// if everyone didn't fill their standups, there are
		// some users who are yet to fill it.
		message := fmt.Sprintf("@%s - a gentle reminder to fill your standup.", strings.Join(usersPendingStandup, ", @"))
		post := &model.Post{
			ChannelId: channelID,
			UserId:    config.GetConfig().BotUserID,
			Type:      model.POST_DEFAULT,
			Message:   message,
		}

		post, appErr := config.Mattermost.CreatePost(post)
		if appErr != nil {
			logger.Error("Error sending window open notification for channel", appErr, map[string]interface{}{"channelID": channelID})
			continue
		}

		err = addReminderPost(post.Id, channelID)
		if err != nil {
			logger.Error("Couldn't add standup reminder posts", err, nil)
			return errors.New(err.Error())
		}

		notificationStatus, err := GetNotificationStatus(channelID)
		if err != nil {
			continue
		}

		notificationStatus.WindowCloseNotificationSent = true
		if err := SetNotificationStatus(channelID, notificationStatus); err != nil {
			return err
		}
	}

	return nil
}

// generateTypeAggregatedStandupReport generates a Type Aggregated standup report
func generateTypeAggregatedStandupReport(
	standupConfig *standup.Config,
	userStandups []*standup.UserStandup,
	membersNoStandup []string,
	channelID string,
	date otime.OTime,
) (*model.Post, error) {
	logger.Debug("Generating type aggregated standup report for channel: "+channelID, nil)

	userTasks := map[string]string{}
	userNoTasks := map[string][]string{}

	for _, userStandup := range userStandups {
		for _, sectionTitle := range standupConfig.Sections {
			userDisplayName, err := getUserDisplayName(userStandup.UserID)
			if err != nil {
				logger.Debug("Couldn't fetch display name for user", err, map[string]string{"userID": userStandup.UserID})
				return nil, err
			}

			header := fmt.Sprintf("##### %s %s", util.UserIcon(userStandup.UserID), userDisplayName)

			if userStandup.Standup[sectionTitle] != nil && len(*userStandup.Standup[sectionTitle]) > 0 {
				userTasks[sectionTitle] += fmt.Sprintf("%s\n1. %s\n", header, strings.Join(*userStandup.Standup[sectionTitle], "\n1. "))
			} else {
				userNoTasks[sectionTitle] = append(userNoTasks[sectionTitle], userDisplayName)
			}
		}
	}

	text := fmt.Sprintf("#### Standup Report for *%s*\n\n", date.Format("2 Jan 2006"))

	if len(userStandups) > 0 {
		if len(membersNoStandup) > 0 {
			text += fmt.Sprintf("%s %s not submitted their standup.\n", strings.Join(membersNoStandup, ", "), util.HasHave(len(membersNoStandup)))
		}

		for _, sectionTitle := range standupConfig.Sections {
			text += "##### ** " + sectionTitle + "**\n\n" + userTasks[sectionTitle] + "\n"
			if len(userNoTasks[sectionTitle]) > 0 {
				text += fmt.Sprintf(
					"%s %s no open items for %s\n",
					strings.Join(userNoTasks[sectionTitle], ", "),
					util.HasHave(len(userNoTasks[sectionTitle])),
					sectionTitle,
				)
			}
		}
	} else {
		text += ":warning: **No user has submitted their standup.**"
	}

	return &model.Post{
		ChannelId: channelID,
		UserId:    config.GetConfig().BotUserID,
		Message:   text,
	}, nil
}

// generateUserAggregatedStandupReport generates a User Aggregated standup report
func generateUserAggregatedStandupReport(
	standupConfig *standup.Config,
	userStandups []*standup.UserStandup,
	membersNoStandup []string,
	channelID string,
	date otime.OTime,
) (*model.Post, error) {
	logger.Debug("Generating user aggregated standup report for channel: "+channelID, nil)

	userTasks := ""

	for _, userStandup := range userStandups {
		userDisplayName, err := getUserDisplayName(userStandup.UserID)
		if err != nil {
			logger.Debug("Couldn't fetch display name for user", err, map[string]string{"userID": userStandup.UserID})
			return nil, err
		}

		header := fmt.Sprintf("#### %s %s", util.UserIcon(userStandup.UserID), userDisplayName)

		userTask := header + "\n\n"

		for _, sectionTitle := range standupConfig.Sections {
			if userStandup.Standup[sectionTitle] == nil || len(*userStandup.Standup[sectionTitle]) == 0 {
				continue
			}

			userTask += fmt.Sprintf("##### %s\n", sectionTitle)
			userTask += "1. " + strings.Join(*userStandup.Standup[sectionTitle], "\n1. ") + "\n\n"
		}

		userTasks += userTask
	}

	text := fmt.Sprintf("#### Standup Report for *%s*\n", date.Format("2 Jan 2006"))

	if len(userStandups) > 0 {
		if len(membersNoStandup) > 0 {
			text += fmt.Sprintf("\n@%s %s not submitted their standup\n\n", strings.Join(membersNoStandup, ", @"), util.HasHave(len(membersNoStandup)))
		}

		text += userTasks
	} else {
		text += ":warning: **No user has submitted their standup.**"
	}

	conf := config.GetConfig()

	return &model.Post{
		ChannelId: channelID,
		UserId:    conf.BotUserID,
		Message:   text,
	}, nil
}

func getUserDisplayName(userID string) (string, error) {
	user, appErr := config.Mattermost.GetUser(userID)
	if appErr != nil {
		return "", errors.New(appErr.Error())
	}
	return user.GetDisplayName(model.SHOW_FULLNAME), nil
}

func addReminderPost(postID string, channelID string) error {
	reminderPosts, err := getReminderPosts(channelID)
	if err != nil {
		return err
	}

	reminderPosts = append(reminderPosts, postID)
	if err := saveReminderPosts(reminderPosts, channelID); err != nil {
		return err
	}

	return nil
}

func getReminderPosts(channelID string) ([]string, error) {
	key := fmt.Sprintf("%s_%s", "reminderPosts", channelID)
	reminderPostsJSON, appErr := config.Mattermost.KVGet(util.GetKeyHash(key))
	if appErr != nil {
		logger.Error("Couldn't get standup reminder posts from KV store", appErr, nil)
		return nil, errors.New(appErr.Error())
	}

	if len(reminderPostsJSON) == 0 {
		return []string{}, nil
	}

	var reminderPosts []string
	if err := json.Unmarshal(reminderPostsJSON, &reminderPosts); err != nil {
		logger.Error("Couldn't unmarshal standup reminder posts", err, nil)
		return nil, err
	}

	return reminderPosts, nil
}

func saveReminderPosts(reminderPosts []string, channelID string) error {
	serializedReminderPosts, err := json.Marshal(reminderPosts)
	if err != nil {
		logger.Error("Couldn't marshal standup reminder posts", err, nil)
		return err
	}

	key := fmt.Sprintf("%s_%s", "reminderPosts", channelID)
	if appErr := config.Mattermost.KVSet(util.GetKeyHash(key), serializedReminderPosts); appErr != nil {
		logger.Error("Couldn't save standup reminder posts into KV store", appErr, nil)
		return errors.New(appErr.Error())
	}

	return nil
}

func deleteReminderPosts(channelID string) error {
	reminderPosts, err := getReminderPosts(channelID)
	if err != nil {
		return err
	}

	// delete reminder posts
	for _, postID := range reminderPosts {
		if appErr := config.Mattermost.DeletePost(postID); appErr != nil {
			logger.Error("Couldn't delete standup reminder post", appErr, nil)
		}
	}

	// deleting KV store entry storing reminder posts for current channel
	key := fmt.Sprintf("%s_%s", "reminderPosts", channelID)
	if appErr := config.Mattermost.KVDelete(util.GetKeyHash(key)); appErr != nil {
		logger.Error("Couldn't delete standup reminder posts from KV store", appErr, nil)
		return errors.New(appErr.Error())
	}

	return nil
}

func isStandupDay(standupConfig *standup.Config) bool {
	todayOtime := otime.Now(standupConfig.Timezone)
	today := time.Date(todayOtime.Year(), todayOtime.Month(), todayOtime.Day(), 0, 0, 0, 0, todayOtime.Location())

	oneMinBeforeToday := today.Add(-1 * time.Minute)
	oneMinAfterToday := today.Add(24 * time.Hour)

	rruleDays := standupConfig.RRule.Between(oneMinBeforeToday, oneMinAfterToday, false)
	return len(rruleDays) > 0
}
