package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/consts"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/handlers"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/slack"
	"github.com/aws/aws-lambda-go/lambda"
)

type inputData struct {
	EventType       string `json:"event_type"`
	ApplicationName string `json:"application_name"`
}

// TODO 起動時のECSのタスク数に関してはTagで指定できるようにする
func handler(ctx context.Context, input inputData) (string, error) {
	fmt.Printf("eventTypes:%s\n", input.EventType)
	fmt.Printf("ApplicationName:%s\n", input.ApplicationName)
	var msg string
	var err error
	switch input.EventType {
	case consts.StartApplicationEventType:
		msg, err = handlers.Start(input.ApplicationName)
	case consts.StopApplicationEventType:
		msg, err = handlers.Stop(input.ApplicationName)
	default:
		msg = "error"
		err = fmt.Errorf("must not reach here!! No handler selected type:%s", input.EventType)
	}

	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		slack.SendMessage(fmt.Sprintf("failed to start/stop %s error:%s", input.ApplicationName, err.Error()))
	} else {
		slack.SendMessage(msg)
	}

	return msg, err
}

func main() {
	if os.Getenv("ENVIRONMENT") == "local" {
		// for local test
		input := inputData{
			EventType:       consts.StartApplicationEventType,
			ApplicationName: "crypto-trading-app",
		}
		handler(context.Background(), input)
	} else {
		lambda.Start(handler)
	}
}
