package notification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/teambition/rrule-go"

	"bou.ke/monkey"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest/mock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/standup-raven/standup-raven/server/config"
	"github.com/standup-raven/standup-raven/server/logger"
	"github.com/standup-raven/standup-raven/server/otime"
	"github.com/standup-raven/standup-raven/server/standup"
	util "github.com/standup-raven/standup-raven/server/utils"

)

var rule *rrule.RRule

const rruleString = "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10"

func init() {
	localRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		panic(err.Error())
	}

	rule = localRule
}

func setUp() *plugintest.API {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI
	return mockAPI
}

func baseMock(mockAPI *plugintest.API) {
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return(nil, nil)
	mockAPI.On("KVSet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1")), mock.Anything).Return(nil)
	mockAPI.On("KVDelete", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return(nil)
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_2"))).Return(nil, nil)
	mockAPI.On("KVSet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_2")), mock.Anything).Return(nil)
	mockAPI.On("KVDelete", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_2"))).Return(nil)
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_3"))).Return(nil, nil)
	mockAPI.On("KVSet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_3")), mock.Anything).Return(nil)
	mockAPI.On("KVDelete", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_3"))).Return(nil)

	monkey.Patch(logger.Debug, func(msg string, err error, keyValuePairs ...interface{}) {})
	monkey.Patch(logger.Error, func(msg string, err error, extraData map[string]interface{}) {})
	monkey.Patch(logger.Info, func(msg string, err error, keyValuePairs ...interface{}) {})
	monkey.Patch(logger.Warn, func(msg string, err error, keyValuePairs ...interface{}) {})

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)
	otime.DefaultLocation = location
}

func TearDown() {
	monkey.UnpatchAll()
}

func TestSendNotificationsAndReports(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		switch {
		case channelID == "channel_1":
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		case channelID == "channel_2":
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		case channelID == "channel_3":
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	parsedRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		switch {
		case channelID == "channel_1":
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
			}
			standupConfig := &standup.Config{
				ChannelID:                  "channel_1",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRule,
			}
			return standupConfig, nil
		case channelID == "channel_2":
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}
			return &standup.Config{
				ChannelID:                  "channel_2",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRule,
			}, nil
		case channelID == "channel_3":
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
			}
			return &standup.Config{
				ChannelID:                  "channel_3",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRule,
			}, nil
		default:
			t.Fatal("unknown argument encountered: " + channelID)
			return nil, nil
		}
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 1)
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 1)
}

func TestSendNotificationsAndReports_NoStandupChannels(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) { return map[string]string{}, nil })

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_GetStandupChannels_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return nil, errors.New("")
	})

	assert.NotNil(t, SendNotificationsAndReports(), "no error should have been produced")
}

func TestSendNotificationsAndReports_SendStandupReport_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return errors.New("")
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	parsedRrule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		if channelID == "channel_1" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
			}

			return &standup.Config{
				ChannelID:                  "channel_1",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRrule,
			}, nil
		} else if channelID == "channel_2" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_2",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRrule,
			}, nil
		} else if channelID == "channel_3" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_3",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRrule,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.NotNil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 1)
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 1)
}

func TestSendNotificationsAndReports_GetNotificationStatus_NoData(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		if channelID == "channel_1" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
			}

			return &standup.Config{
				ChannelID:                  "channel_1",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRRule,
			}, nil
		} else if channelID == "channel_2" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_2",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRRule,
			}, nil
		} else if channelID == "channel_3" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_3",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRRule,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 1)
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 1)
}

func TestSendNotificationsAndReports_GetUser_Error(t *testing.T) {
	// TODO - fix this flaky test
	t.SkipNow()

	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(nil, &model.AppError{})

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-55 * time.Minute),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      parsedRRule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return nil, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	err = SendNotificationsAndReports()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	assert.NotNil(t, err, "error should have been produced: "+msg)
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
	mockAPI.AssertNumberOfCalls(t, "KVGet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 0)
}

func TestSendNotificationsAndReports_GetStandupConfig_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(nil, &model.AppError{})

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return nil, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	monkey.Patch(config.Mattermost.GetUser, func(string) (*model.User, *model.AppError) {
		return nil, model.NewAppError("", "", nil, "", http.StatusInternalServerError)
	})

	assert.NotNil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_GetStandupConfig_Nil(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return nil, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced as no standup config found is handled")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_WindowOpenNotificationSent_Sent(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		if channelID == "channel_1" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
			}

			return &standup.Config{
				ChannelID:                  "channel_1",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRRule,
			}, nil
		} else if channelID == "channel_2" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_2",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRRule,
			}, nil
		} else if channelID == "channel_3" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_3",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      parsedRRule,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
}

func TestSendNotificationsAndReports_NotWorkDay(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-50*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:       "channel_1",
			WindowOpenTime:  windowOpenTime,
			WindowCloseTime: windowCloseTime,
			Enabled:         true,
			Members:         []string{"user_id_1", "user_id_2"},
			ReportFormat:    config.ReportFormatUserAggregated,
			Sections:        []string{"section 1", "section 2"},
			Timezone:        "Asia/Kolkata",
			RRuleString:     "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:           parsedRRule,
		}, nil
	})

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      parsedRRule,
		}, nil
	})

	config.SetConfig(mockConfig)

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_Integration(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("KVSet", mock.AnythingOfType("string"), mock.Anything).Return(nil)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)
	mockAPI.On("KVGet", "uScyewRiWEwQavauYw9iOK76jISl+5Qq0mV+Cn/jFPs=").Return(
		[]byte("{\"channel_1\": \":channel_1\"}"), nil,
	)
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s_%s", config.CacheKeyPrefixNotificationStatus, "channel_1", util.GetCurrentDateString("Asia/Kolkata")))).Return(nil, nil)

	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	windowOpenTime := otime.OTime{
		Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
	}
	windowCloseTime := otime.OTime{
		Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
	}
	standupConfig, _ := json.Marshal(&standup.Config{
		ChannelID:                  "channel_1",
		WindowOpenTime:             windowOpenTime,
		WindowCloseTime:            windowCloseTime,
		Enabled:                    true,
		Members:                    []string{"user_id_1", "user_id_2"},
		ReportFormat:               config.ReportFormatUserAggregated,
		Sections:                   []string{"section 1", "section 2"},
		Timezone:                   "Asia/Kolkata",
		WindowOpenReminderEnabled:  true,
		WindowCloseReminderEnabled: true,
		RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
		RRule:                      parsedRRule,
	})
	mockAPI.On("KVGet", "UzFgbepiypG8qfVARBfHu154LDNiZOw7Mr6Ue4kNZrk=").Return(standupConfig, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 1)
}

func TestSendNotificationsAndReports_sendWindowCloseNotification_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(nil, util.EmptyAppError())
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_2",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      parsedRRule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{}, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_FilterChannelNotifications_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(nil, util.EmptyAppError())
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_2",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      rule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return nil, errors.New("")
	})

	err := SendNotificationsAndReports()
	if err != nil {
		assert.NotNil(t, err, err.Error())
	}
}

func TestSendNotificationsAndReports_Standup_Disabled(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    false,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
}

func TestSendNotificationsAndReports_StandupReport_Sent(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           true,
			WindowOpenNotificationSent:  true,
			WindowCloseNotificationSent: true,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      rule,
		}, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
}

func TestSendNotificationsAndReports_SendWindowOpenNotification_CreatePost_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, model.NewAppError("", "", nil, "", 0))
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=50",
			RRule:                      rule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{}, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 1)
}

func TestSendNotificationsAndReports_ShouldSendWindowOpenNotification_NotYet(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(2 * time.Hour),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      rule,
		}, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_WindowCloseNotification(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-55 * time.Minute),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
			RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
			RRule:                      rule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 1)
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 0)
}

func TestGetNotificationStatus(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	notificationStatusJSON, _ := json.Marshal(ChannelNotificationStatus{
		WindowOpenNotificationSent:  true,
		WindowCloseNotificationSent: false,
		StandupReportSent:           true,
	})
	mockAPI.On("KVGet", mock.AnythingOfType("string")).Return(notificationStatusJSON, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	actualNotificationStatus, err := GetNotificationStatus("channel_1")
	assert.Nil(t, err, "no error should have been produced")

	expectedNotificationStatus := &ChannelNotificationStatus{
		WindowOpenNotificationSent:  true,
		WindowCloseNotificationSent: false,
		StandupReportSent:           true,
	}
	assert.Equal(t, expectedNotificationStatus, actualNotificationStatus)
}

func TestGetNotificationStatus_KVGet_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("KVGet", mock.AnythingOfType("string")).Return(nil, model.NewAppError("", "", nil, "", 0))

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	actualNotificationStatus, err := GetNotificationStatus("channel_1")
	assert.NotNil(t, err, "error should have been produced as KVGet failed")
	assert.Nil(t, actualNotificationStatus)
}

func TestGetNotificationStatus_Json_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	notificationStatusJSON, _ := json.Marshal(ChannelNotificationStatus{
		WindowOpenNotificationSent:  true,
		WindowCloseNotificationSent: false,
		StandupReportSent:           true,
	})
	mockAPI.On("KVGet", mock.AnythingOfType("string")).Return(notificationStatusJSON[0:len(notificationStatusJSON)-10], nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	actualNotificationStatus, err := GetNotificationStatus("channel_1")
	assert.NotNil(t, err, "error should have been produced as inbalid JSOn was returned by KVGet")
	assert.Nil(t, actualNotificationStatus)
}

func TestGetNotificationStatus_KVSet_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	notificationStatusJSON, _ := json.Marshal(ChannelNotificationStatus{
		WindowOpenNotificationSent:  true,
		WindowCloseNotificationSent: false,
		StandupReportSent:           true,
	})
	mockAPI.On("KVGet", mock.AnythingOfType("string")).Return(notificationStatusJSON, nil)
	mockAPI.On("KVSet")

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	actualNotificationStatus, err := GetNotificationStatus("channel_1")
	assert.Nil(t, err, "no error should have been produced")

	expectedNotificationStatus := &ChannelNotificationStatus{
		WindowOpenNotificationSent:  true,
		WindowCloseNotificationSent: false,
		StandupReportSent:           true,
	}
	assert.Equal(t, expectedNotificationStatus, actualNotificationStatus)
}

func TestSendStandupReport(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// no standup channels specified
	err = SendStandupReport([]string{}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// error in GetStandupConfig
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig failed")

	// no standup config
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig didn't return any standup config")

	// standup with no members
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "shouldn't produce error as standup with no members is a valid case")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 4)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 4)
}

func TestSendStandupReport_GetReminderPosts_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return(nil, model.NewAppError("", "", nil, "", 0))
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 0)
}

func TestSendStandupReport_DeleteReminderPosts_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	mockAPI.On("KVDelete", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return(nil, model.NewAppError("", "", nil, "", 0))
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 1)
}

func TestSendStandupReport_GetReminderPosts_DeletePostError(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return([]byte("[\"post-id-1\"]"), nil)
	mockAPI.On("DeletePost", "post-id-1").Return(model.NewAppError("", "", nil, "", 0))
	baseMock(mockAPI)

	mockAPI.On("DeletePost", "post-id-1").Return(nil)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 1)
}

func TestSendStandupReport_GetReminderPosts_KVDeleteError(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return([]byte("[\"post-id-1\"]"), nil)
	mockAPI.On("KVDelete", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return(model.NewAppError("", "", nil, "", 0))
	baseMock(mockAPI)

	mockAPI.On("DeletePost", "post-id-1").Return(nil)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 1)
}

func TestSendStandupReport_GetReminderPosts_JsonError(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return([]byte("{"), nil)
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 0)
}

func TestSendStandupReport_GetReminderPosts_Data(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	mockAPI.On("KVGet", util.GetKeyHash(fmt.Sprintf("%s_%s", "reminderPosts", "channel_1"))).Return([]byte("[\"post-id-1\"]"), nil)
	baseMock(mockAPI)

	mockAPI.On("DeletePost", "post-id-1").Return(nil)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 1)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 1)
}

func TestSendStandupReport_GetUserStandup_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, errors.New("")
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetUserStandup failed")
}

func TestSendStandupReport_GetUserStandup_Nil(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Return()
	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// no standup channels specified
	err = SendStandupReport([]string{}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// error in GetStandupConfig
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig failed")

	// no standup config
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig didn't return any standup config")

	// standup with no members
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "shouldn't produce error as standup with no members is a valid case")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 4)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 4)
}

func TestSendStandupReport_GetUserStandup_Nil_GetUser_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Return()

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		nil, model.NewAppError("", "", nil, "", 0),
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetUser failed")
}

func TestSendStandupReport_ReportFormatUserAggregated(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// no standup channels specified
	err = SendStandupReport([]string{}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// error in GetStandupConfig
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig failed")

	// no standup config
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig didn't return any standup config")

	// standup with no members
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "shouldn't produce error as standup with no members is a valid case")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 4)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 4)
}

func TestSendStandupReport_UnknownReportFormat(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               "some_unknown_report_format",
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.NotNil(t, err, "should produce error as report format was unknown")
}

func TestSendStandupReport_ReportVisibility_Public(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("CreatePost", mock.AnythingOfType("*model.Post"), mock.Anything).Return(&model.Post{}, nil)

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// no standup channels specified
	err = SendStandupReport([]string{}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// error in GetStandupConfig
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig failed")

	// no standup config
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig didn't return any standup config")

	// standup with no members
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.Nil(t, err, "shouldn't produce error as standup with no members is a valid case")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 4)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 4)
}

func TestSendStandupReport_ReportVisibility_Public_CreatePost_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("CreatePost", mock.AnythingOfType("*model.Post"), mock.Anything).Return(nil, model.NewAppError("", "", nil, "", 0))

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.NotNil(t, err, "should not produce any error")

	// no standup channels specified
	err = SendStandupReport([]string{}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// error in GetStandupConfig
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig failed")

	// no standup config
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig didn't return any standup config")

	// standup with no members
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", false)
	assert.NotNil(t, err, "shouldn't produce error as standup with no members is a valid case")
}

func TestSendStandupReport_UpdateStatus_True(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", true)
	assert.Nil(t, err, "should not produce any error")

	// no standup channels specified
	err = SendStandupReport([]string{}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", false)
	assert.Nil(t, err, "should not produce any error")

	// error in GetStandupConfig
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", true)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig failed")

	// no standup config
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", true)
	assert.NotNil(t, err, "should produce any error as GetStandupConfig didn't return any standup config")

	// standup with no members
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", true)
	assert.Nil(t, err, "shouldn't produce error as standup with no members is a valid case")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 4)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 4)
}

func TestSendStandupReport_UpdateStatus_True_GetNotificationStatus_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	mockAPI.On("GetUser", "user_id_1").Return(
		&model.User{
			FirstName: "Foo",
			LastName:  "Bar",
		}, nil,
	)

	mockAPI.On("GetUser", "user_id_2").Return(
		&model.User{
			FirstName: "John",
			LastName:  "Doe",
		}, nil,
	)

	mockAPI.On("SendEphemeralPost", mock.AnythingOfType("string"), mock.Anything).Return(&model.Post{})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  channelID,
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			ReportFormat:               config.ReportFormatTypeAggregated,
			Sections:                   []string{"section_1", "section_2"},
			Members:                    []string{"user_id_1", "user_id_2"},
			Enabled:                    true,
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return &standup.UserStandup{
			UserID:    userID,
			ChannelID: channelID,
			Standup: map[string]*[]string{
				"section_1": {"task_1", "task_2"},
				"section_2": {"task_3", "task_4"},
			},
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return nil, errors.New("")
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return nil
	})

	err := SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", true)
	assert.Nil(t, err, "should not produce any error")

	monkey.Unpatch(GetNotificationStatus)
	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})
	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		return errors.New("")
	})

	err = SendStandupReport([]string{"channel_1", "channel_2"}, otime.Now("Asia/Kolkata"), ReportVisibilityPrivate, "user_1", true)
	assert.NotNil(t, err, "should not produce any error")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 3)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 3)
}

func TestSetNotificationStatus(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("KVSet", mock.AnythingOfType("string"), mock.Anything).Return(nil)
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_id_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	assert.Nil(t, SetNotificationStatus("channel_id_1", &ChannelNotificationStatus{}))
}

func TestSetNotificationStatus_JsonMarshal_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)

	monkey.Patch(json.Marshal, func(v interface{}) ([]byte, error) {
		return nil, errors.New("")
	})
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_id_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})

	assert.NotNil(t, SetNotificationStatus("channel_id_1", &ChannelNotificationStatus{}))
}

func TestSetNotificationStatus_KVSet_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("KVSet", mock.AnythingOfType("string"), mock.Anything).Return(model.NewAppError("", "", nil, "", 0))
	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_id_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: true,
		}, nil
	})
	assert.NotNil(t, SetNotificationStatus("channel_id_1", &ChannelNotificationStatus{}))
}

func TestSendNotificationsAndReports_GetUserStandup_Nodata(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		if channelID == "channel_1" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_1",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      rule,
			}, nil
		} else if channelID == "channel_2" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_2",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                "FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10",
				RRule:                      rule,
			}, nil
		} else if channelID == "channel_3" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_3",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                rruleString,
				RRule:                      rule,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})
	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, nil
	})
	err := SendStandupReport([]string{"channel_1", "channel_2", "channel_3"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", true)
	assert.Nil(t, err, "should not produce any error")
	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 3)
	mockAPI.AssertNumberOfCalls(t, "KVGet", 3)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 0)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 3)
}

func TestSendNotificationsAndReports_MemberNoStandup(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  false,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: false,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		if channelID == "channel_1" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_1",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatTypeAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                rruleString,
				RRule:                      rule,
			}, nil
		} else if channelID == "channel_2" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_2",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                rruleString,
				RRule:                      rule,
			}, nil
		} else if channelID == "channel_3" {
			windowOpenTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(-1 * time.Hour),
			}
			windowCloseTime := otime.OTime{
				Time: otime.Now("Asia/Kolkata").Add(1 * time.Minute),
			}

			return &standup.Config{
				ChannelID:                  "channel_3",
				WindowOpenTime:             windowOpenTime,
				WindowCloseTime:            windowCloseTime,
				Enabled:                    true,
				Members:                    []string{"user_id_1", "user_id_2"},
				ReportFormat:               config.ReportFormatUserAggregated,
				Sections:                   []string{"section 1", "section 2"},
				Timezone:                   "Asia/Kolkata",
				WindowOpenReminderEnabled:  true,
				WindowCloseReminderEnabled: true,
				RRuleString:                rruleString,
				RRule:                      rule,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" {
				return nil, nil
			} else if userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" {
				return nil, nil
			} else if userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})
	err := SendStandupReport([]string{"channel_1", "channel_2", "channel_3"}, otime.Now("Asia/Kolkata"), ReportVisibilityPublic, "user_1", true)
	assert.Nil(t, err, "should not produce any error")
	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "KVGet", 5)
	mockAPI.AssertNumberOfCalls(t, "KVSet", 2)
	mockAPI.AssertNumberOfCalls(t, "KVDelete", 3)
}

func TestSendNotificationsAndReports_StandupConfig_Error(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(filterChannelNotification, func(channels map[string]string) ([]string, []string, []string, error) {
		return []string{}, []string{}, []string{"channel_1", "channel_2", "channel_3"}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, errors.New("")
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return nil, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.NotNil(t, SendNotificationsAndReports())
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_StandupConfig_Nil(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
			"channel_2": "channel_2",
			"channel_3": "channel_3",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		if channelID == "channel_1" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		} else if channelID == "channel_2" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		} else if channelID == "channel_3" {
			return &ChannelNotificationStatus{
				StandupReportSent:           false,
				WindowOpenNotificationSent:  true,
				WindowCloseNotificationSent: true,
			}, nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil, nil
	})
	monkey.Patch(filterChannelNotification, func(channels map[string]string) ([]string, []string, []string, error) {
		return []string{}, []string{}, []string{"channel_1", "channel_2", "channel_3"}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		return nil, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		if channelID == "channel_1" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return nil, nil
			}
		} else if channelID == "channel_2" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		} else if channelID == "channel_3" {
			if userID == "user_id_1" || userID == "user_id_2" {
				return &standup.UserStandup{}, nil
			}
		}

		panic(t)
	})

	assert.NotNil(t, SendNotificationsAndReports())
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_WindowCloseReminderEnabled_Disabled(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  true,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-55 * time.Minute),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  true,
			WindowCloseReminderEnabled: false,
			RRuleString:                rruleString,
			RRule:                      rule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestSendNotificationsAndReports_WindowOpenReminderEnabled_Disabled(t *testing.T) {
	defer TearDown()
	mockAPI := setUp()
	baseMock(mockAPI)
	mockAPI.On("CreatePost", mock.AnythingOfType(model.Post{}.Type)).Return(&model.Post{}, nil)
	mockAPI.On("GetUser", mock.AnythingOfType("string")).Return(&model.User{Username: "username"}, nil)

	location, _ := time.LoadLocation("Asia/Kolkata")
	mockConfig := &config.Configuration{
		Location: location,
	}

	config.SetConfig(mockConfig)

	monkey.Patch(standup.GetStandupChannels, func() (map[string]string, error) {
		return map[string]string{
			"channel_1": "channel_1",
		}, nil
	})

	monkey.Patch(SendStandupReport, func(channelIDs []string, date otime.OTime, visibility string, userId string, updateStatus bool) error {
		return nil
	})

	monkey.Patch(GetNotificationStatus, func(channelID string) (*ChannelNotificationStatus, error) {
		return &ChannelNotificationStatus{
			StandupReportSent:           false,
			WindowOpenNotificationSent:  false,
			WindowCloseNotificationSent: false,
		}, nil
	})

	monkey.Patch(standup.GetStandupConfig, func(channelID string) (*standup.Config, error) {
		windowOpenTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(-1 * time.Minute),
		}
		windowCloseTime := otime.OTime{
			Time: otime.Now("Asia/Kolkata").Add(5 * time.Minute),
		}

		return &standup.Config{
			ChannelID:                  "channel_1",
			WindowOpenTime:             windowOpenTime,
			WindowCloseTime:            windowCloseTime,
			Enabled:                    true,
			Members:                    []string{"user_id_1", "user_id_2"},
			ReportFormat:               config.ReportFormatUserAggregated,
			Sections:                   []string{"section 1", "section 2"},
			Timezone:                   "Asia/Kolkata",
			WindowOpenReminderEnabled:  false,
			WindowCloseReminderEnabled: true,
			RRuleString:                rruleString,
			RRule:                      rule,
		}, nil
	})

	monkey.Patch(SetNotificationStatus, func(channelID string, status *ChannelNotificationStatus) error {
		if channelID == "channel_1" {
			return nil
		} else if channelID == "channel_2" {
			return nil
		} else if channelID == "channel_3" {
			return nil
		}

		t.Fatal("unknown argument encountered: " + channelID)
		return nil
	})

	monkey.Patch(standup.GetUserStandup, func(userID, channelID string, date otime.OTime) (*standup.UserStandup, error) {
		return nil, nil
	})

	assert.Nil(t, SendNotificationsAndReports(), "no error should have been produced")
	mockAPI.AssertNumberOfCalls(t, "CreatePost", 0)
}

func TestIsStandupDay(t *testing.T) {
	parsedRRule, err := util.ParseRRuleFromString("FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR,SA,SU;COUNT=10", time.Now().Add(-5*24*time.Hour))
	if err != nil {
		t.Fatal("Couldn't parse RRULE", err)
		return
	}

	standupConfig := &standup.Config{
		Timezone: "Asia/Kolkata",
		RRule:    parsedRRule,
	}

	isStandupDay(standupConfig)
}
