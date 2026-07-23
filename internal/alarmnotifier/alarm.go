package alarmnotifier

import (
	"encoding/json"
	"fmt"
	"strings"
)

type AlarmNotification struct {
	AlarmName        string `json:"AlarmName"`
	AlarmDescription string `json:"AlarmDescription"`
	AWSAccountID     string `json:"AWSAccountId"`
	NewStateValue    string `json:"NewStateValue"`
	NewStateReason   string `json:"NewStateReason"`
	StateChangeTime  string `json:"StateChangeTime"`
	Region           string `json:"Region"`
	OldStateValue    string `json:"OldStateValue"`
}

func ParseAlarmMessage(raw string) (*AlarmNotification, error) {
	var alarm AlarmNotification

	if err := json.Unmarshal([]byte(raw), &alarm); err != nil {
		return nil, fmt.Errorf("failed to parse CW alarm message: %w", err)
	}

	if alarm.AlarmName == "" {
		return nil, fmt.Errorf("CW alarm message missing required field AlarmName")
	}
	return &alarm, nil
}

func FormatPlainText(alarm *AlarmNotification, env string) string {
	var b strings.Builder
	b.WriteString("CloudWatch alarm: ")
	b.WriteString(alarm.NewStateValue)
	b.WriteByte('\n')
	b.WriteString("Alarm: ")
	b.WriteString(alarm.AlarmName)
	if env != "" {
		b.WriteString("\nEnv: ")
		b.WriteString(env)
	}
	if alarm.Region != "" {
		b.WriteString("\nRegion: ")
		b.WriteString(alarm.Region)
	}
	if alarm.OldStateValue != "" {
		b.WriteString("\nPrevious: ")
		b.WriteString(alarm.OldStateValue)
	}
	if alarm.StateChangeTime != "" {
		b.WriteString("\nTime: ")
		b.WriteString(alarm.StateChangeTime)
	}
	if alarm.AlarmDescription != "" {
		b.WriteString("\nDescription: ")
		b.WriteString(alarm.AlarmDescription)
	}
	if alarm.NewStateReason != "" {
		b.WriteString("\nReason: ")
		b.WriteString(alarm.NewStateReason)
	}
	return b.String()
}

func FormatHTML(alarm *AlarmNotification, env string) string {
	var b strings.Builder
	b.WriteString("<b>CloudWatch alarm: ")
	b.WriteString(escapeHTML(alarm.NewStateValue))
	b.WriteString("</b>\n")
	b.WriteString("<b>Alarm:</b> ")
	b.WriteString(escapeHTML(alarm.AlarmName))
	if env != "" {
		b.WriteString("\n<b>Env:</b> ")
		b.WriteString(escapeHTML(env))
	}
	if alarm.Region != "" {
		b.WriteString("\n<b>Region:</b> ")
		b.WriteString(escapeHTML(alarm.Region))
	}
	if alarm.OldStateValue != "" {
		b.WriteString("\n<b>Previous:</b> ")
		b.WriteString(escapeHTML(alarm.OldStateValue))
	}
	if alarm.StateChangeTime != "" {
		b.WriteString("\n<b>Time:</b> ")
		b.WriteString(escapeHTML(alarm.StateChangeTime))
	}
	if alarm.AlarmDescription != "" {
		b.WriteString("\n<b>Description:</b> ")
		b.WriteString(escapeHTML(alarm.AlarmDescription))
	}
	if alarm.NewStateReason != "" {
		b.WriteString("\n<b>Reason:</b> ")
		b.WriteString(escapeHTML(alarm.NewStateReason))
	}
	return b.String()
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(value)
}

func discordColor(state string) int {
	switch strings.ToUpper(state) {
	case "ALARM":
		return 0xed4245
	case "OK":
		return 0x57f287
	default:
		return 0xfee75c
	}
}
