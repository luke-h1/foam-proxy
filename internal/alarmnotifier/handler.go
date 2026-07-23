package alarmnotifier

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

func (n *Notifier) HandleSNS(ctx context.Context, event events.SNSEvent) error {
	if len(event.Records) == 0 {
		return fmt.Errorf("sns event has no records")
	}

	var firstError error

	for _, record := range event.Records {
		alarm, err := ParseAlarmMessage(record.SNS.Message)
		if err != nil {
			log.Printf("skipping sns record: %v", err)
			if firstError == nil {
				firstError = err
			}
			continue
		}
		log.Printf("alarm received %s -> %s for reason -> %s", alarm.AlarmName, alarm.NewStateValue, alarm.NewStateReason)

		// only notify on abnormal transitions; skip recoveries (OK) and INSUFFICIENT_DATA
		if !strings.EqualFold(alarm.NewStateValue, "ALARM") {
			log.Printf("skipping non-alarm state %s for %s", alarm.NewStateValue, alarm.AlarmName)
			continue
		}

		if err := n.Notify(ctx, alarm); err != nil {
			log.Printf("Notification failed for %s: %v", alarm.AlarmName, err)
			if firstError == nil {
				firstError = err
			}
		}
	}
	return firstError
}
