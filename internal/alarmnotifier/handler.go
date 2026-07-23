package alarmnotifier

import (
	"context"
	"fmt"
	"log"

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

		if err := n.Notify(ctx, alarm); err != nil {
			log.Printf("Notification failed for %s: %v", alarm.AlarmName, err)
			if firstError == nil {
				firstError = err
			}
		}
	}
	return firstError
}
