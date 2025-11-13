package utils

import (
	"strings"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/consts"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/rds"
)

func IsInTarget[T ec2.Tag | rds.Tag | ecs.Tag](tags []*T, eventType string) bool {
	if GetTargetStatus(tags, eventType) == consts.InTargetStatus {
		return true
	}
	return false
}

func GetTargetStatus[T ec2.Tag | rds.Tag | ecs.Tag](tags []*T, eventType string) consts.TargetStatus {
	for _, tag := range tags {
		var key string
		var value string
		switch (interface{})(tag).(type) {
		case *ec2.Tag:
			key = *(interface{})(tag).(*ec2.Tag).Key
			value = *(interface{})(tag).(*ec2.Tag).Value
		case *rds.Tag:
			key = *(interface{})(tag).(*rds.Tag).Key
			value = *(interface{})(tag).(*rds.Tag).Value
		case *ecs.Tag:
			key = *(interface{})(tag).(*ecs.Tag).Key
			value = *(interface{})(tag).(*ecs.Tag).Value
		}
		if key == consts.TargetTagKey {
			lowerValue := strings.ToLower(value)
			if lowerValue == consts.InTarget {
				return consts.InTargetStatus
			} else if lowerValue == consts.StopOnly && eventType == consts.StopApplicationEventType {
				return consts.InTargetStatus
			} else {
				return consts.NotInTargetStatus
			}
		}
	}
	return consts.NoStatusStatus
}
