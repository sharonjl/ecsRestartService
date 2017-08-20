package main

import (
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
	app.UsageText = "ecsRestartService [global options] [service1 service2 ...]"
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
			Usage: "list of services to restart",
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

	tds, err := getTaskDefinitions(svc, services)
	if err != nil {
		return err
	}

	for serviceName, td := range tds {
		newTaskDef, err := cloneTaskDefinition(svc, td)
		if err != nil {
			return err
		}
		updateService(svc, serviceName, newTaskDef)
	}
	return nil
}

func getTaskDefinitions(svc *ecs.ECS, services []string) (map[string]*ecs.TaskDefinition, error) {
	input := &ecs.DescribeServicesInput{
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

func updateService(svc *ecs.ECS, serviceName string, task *ecs.TaskDefinition) error {
	input := &ecs.UpdateServiceInput{
		Service:        aws.String(serviceName),
		TaskDefinition: task.TaskDefinitionArn,
	}
	_, err := svc.UpdateService(input)
	return err
}
