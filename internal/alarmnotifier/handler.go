package alarmnotifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

func (n *Notifier) HandleSNS(ctx context.Context, event events.SNSEvent) error {
	if len(event.Records) == 0 {
		return errors.New("sns event has no records")
	}

	errs := []error{}
	for _, record := range event.Records {
		alarm, err := ParseAlarmMessage(record.SNS.Message)
		if err != nil {
			errs = append(errs, fmt.Errorf("parse sns record: %w", err))
			continue
		}
		slog.Info("alarm received",
			"alarm", alarm.AlarmName,
			"state", alarm.NewStateValue,
			"reason", alarm.NewStateReason,
		)

		// only notify on abnormal transitions; skip recoveries (OK) and INSUFFICIENT_DATA
		if !strings.EqualFold(alarm.NewStateValue, "ALARM") {
			slog.Info("skipping non-alarm state", "alarm", alarm.AlarmName, "state", alarm.NewStateValue)
			continue
		}

		if err := n.Notify(ctx, alarm); err != nil {
			errs = append(errs, fmt.Errorf("notify %s: %w", alarm.AlarmName, err))
		}
	}
	return errors.Join(errs...)
}
