package handlers

import (
	"fmt"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/consts"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

func Stop(appName string) (string, error) {
	fmt.Println("Stopping Application!!")
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	rdsSvc = rds.New(sess, aws.NewConfig().WithRegion("ap-northeast-1"))

	err := stopRDS()
	if err != nil {
		return "failed to stop rds", err
	}

	fmt.Println("Stopped Application!!")
	return fmt.Sprintf("%s stopped!!", appName), nil
}

func stopRDS() error {
	dbInstances, err := rdsSvc.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
	if err != nil {
		return err
	}

	fmt.Println("Stopping RDS instances...")
	for _, dbInstance := range dbInstances.DBInstances {
		isTarget := utils.IsInTarget(dbInstance.TagList, consts.StopApplicationEventType)
		if !isTarget {
			continue
		}
		if *dbInstance.DBInstanceStatus == "available" {
			_, err := rdsSvc.StopDBInstance(&rds.StopDBInstanceInput{
				DBInstanceIdentifier: dbInstance.DBInstanceIdentifier,
			})
			fmt.Printf("Instance: %s\n", *dbInstance.DBInstanceIdentifier)
			if err != nil {
				return err
			}
		}
	}
	fmt.Println("Sent RDS stop request!")
	return nil
}
