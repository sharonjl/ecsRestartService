package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/urfave/cli"
)

var (
	sess *session.Session
	svc  *ecs.ECS
)

func main() {
	app := cli.NewApp()
	app.Name = "ecsRestartService"
	app.Usage = "Simplifying ECS deployment and management"
	app.UsageText = "ecsRestartService --cluster default --region us-west-1 --services service1,service2"
	app.Description = ""
	app.Version = ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "cluster",
			Usage: "the ECS cluster on which the services reside",
		},
		cli.StringFlag{
			Name:  "region",
			Usage: "aws region",
		},
		cli.StringSliceFlag{
			Name:  "services",
			Usage: "services to restart",
		},
	}
	app.Before = prepare
	app.Action = restartServices

	err := app.Run(os.Args)
	if err != nil {
		println(err.Error())
	}
}

func prepare(c *cli.Context) error {
	s, err := session.NewSession(
		&aws.Config{
			Region: aws.String(c.String("region")),
		})
	if err != nil {
		return err
	}
	sess = s
	svc = ecs.New(sess)
	return nil
}

func restartServices(c *cli.Context) error {
	services := c.StringSlice("services")
	if len(services) == 0 {
		println("No services defined")
		cli.ShowAppHelp(c)
		return nil
	}

	cluster := c.String("cluster")
	if len(cluster) == 0 {
		println("No cluster defined")
		cli.ShowAppHelp(c)
		return nil
	}

	log.Print("Querying TaskDefinitions for services.")
	tds, err := getTaskDefinitions(svc, cluster, services)
	if err != nil {
		return err
	}

	for serviceName, td := range tds {
		newTaskDef, err := cloneTaskDefinition(svc, td)
		if err != nil {
			return err
		}

		log.Printf("Updating %s to %s", serviceName, aws.StringValue(newTaskDef.TaskDefinitionArn))
		err = updateService(svc, cluster, serviceName, newTaskDef)
		if err != nil {
			return err
		}
	}
	return nil
}

func getTaskDefinitions(svc *ecs.ECS, cluster string, services []string) (map[string]*ecs.TaskDefinition, error) {
	input := &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: aws.StringSlice(services),
	}
	m := make(map[string]*ecs.TaskDefinition)

	result, err := svc.DescribeServices(input)
	if err != nil {
		return nil, err
	}

	for _, s := range result.Services {
		td, err := getTaskDefinition(svc, s.Deployments[0].TaskDefinition)
		if err != nil {
			return nil, err
		}
		m[aws.StringValue(s.ServiceName)] = td
	}
	return m, nil
}

func getTaskDefinition(svc *ecs.ECS, name *string) (*ecs.TaskDefinition, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: name,
	}

	result, err := svc.DescribeTaskDefinition(input)
	if err != nil {
		return nil, err
	}
	return result.TaskDefinition, nil
}

func cloneTaskDefinition(svc *ecs.ECS, td *ecs.TaskDefinition) (*ecs.TaskDefinition, error) {
	input := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions: td.ContainerDefinitions,
		Family:               td.Family,
		NetworkMode:          td.NetworkMode,
		PlacementConstraints: td.PlacementConstraints,
		Volumes:              td.Volumes,
		TaskRoleArn:          td.TaskRoleArn,
	}

	result, err := svc.RegisterTaskDefinition(input)
	if err != nil {
		return nil, err
	}
	return result.TaskDefinition, nil
}

func updateService(svc *ecs.ECS, cluster string, serviceName string, task *ecs.TaskDefinition) error {
	input := &ecs.UpdateServiceInput{
		Cluster:        aws.String(cluster),
		Service:        aws.String(serviceName),
		TaskDefinition: task.TaskDefinitionArn,
	}
	_, err := svc.UpdateService(input)
	return err
}
