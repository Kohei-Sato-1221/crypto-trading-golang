package handlers

import (
	"fmt"

	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/consts"
	"github.com/Kohei-Sato-1221/crypto-trading-golang/lambda/aws-scheduler/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
)

var rdsSvc *rds.RDS

func Start(appName string) (string, error) {
	fmt.Println("Starting Application!!")

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	rdsSvc = rds.New(sess, aws.NewConfig().WithRegion("ap-northeast-1"))

	err, _ := startRDS()
	if err != nil {
		return "failed to start rds", err
	}
	return "success", nil
}

func startRDS() (error, int) {
	dbInstances, err := rdsSvc.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
	if err != nil {
		return err, 0
	}

	fmt.Println("Starting RDS instances...")
	rdsIntanceCnt := 0
	for _, dbInstance := range dbInstances.DBInstances {
		isTarget := utils.IsInTarget(dbInstance.TagList, consts.StartApplicationEventType)
		if !isTarget {
			continue
		}
		if *dbInstance.DBInstanceStatus == "stopped" {
			_, err := rdsSvc.StartDBInstance(&rds.StartDBInstanceInput{
				DBInstanceIdentifier: dbInstance.DBInstanceIdentifier,
			})
			fmt.Printf("Instance: %s\n", *dbInstance.DBInstanceIdentifier)
			if err != nil {
				return err, 0
			}
			rdsIntanceCnt++
		}
	}
	fmt.Println("Sent RDS start request!")
	return nil, rdsIntanceCnt
}
