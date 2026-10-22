package control

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecst "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fogo-sh/insta-game/internal/cloud"
)

type AWS struct {
	Config    cloud.Config
	Store     *Store
	formation formationAPI
	tasks     tasksAPI
	network   networkAPI
	prefix    string
}

func NewAWS(ctx context.Context, config cloud.Config, store *Store) (*AWS, error) {
	identity, err := arn.Parse(config.Stack)
	if err != nil {
		return nil, err
	}
	resource := strings.Split(identity.Resource, "/")
	if len(resource) != 3 || resource[0] != "stack" {
		return nil, fmt.Errorf("expected a CloudFormation environment ARN")
	}
	sdk, err := cloud.LoadSDK(ctx, config.Region)
	if err != nil {
		return nil, err
	}
	if _, err := cloud.DescribeStack(ctx, cloudformation.NewFromConfig(sdk), config.Stack, "environment", ""); err != nil {
		return nil, err
	}
	return &AWS{Config: config, Store: store, formation: cloudformation.NewFromConfig(sdk), tasks: ecs.NewFromConfig(sdk), network: ec2.NewFromConfig(sdk), prefix: resource[1] + "-game-"}, nil
}

func (a *AWS) stack(ctx context.Context, i Instance) (*cf.Stack, error) {
	if i.AWS.Environment != "" && i.AWS.Environment != a.Config.Stack {
		return nil, fmt.Errorf("instance belongs to another AWS environment")
	}
	return cloud.DescribeStack(ctx, a.formation, a.prefix+owner(i)[:24], "instance", owner(i))
}

func (a *AWS) Ensure(ctx context.Context, i Instance) (Observation, error) {
	if _, err := a.stack(ctx, i); err == nil {
		return a.Inspect(ctx, i)
	} else if !cloud.MissingStack(err) {
		return Observation{}, err
	}
	err := a.Store.Update(i.ID, func(current *Instance) error {
		if current.AWS.Environment != "" && current.AWS.Environment != a.Config.Stack {
			return fmt.Errorf("instance belongs to another AWS environment")
		}
		if current.AWS.Environment == "" {
			current.AWS.Environment = a.Config.Stack
			current.AWS.DataURL = os.Getenv(i.Game.DataURLEnv)
		}
		i.AWS = current.AWS
		return nil
	})
	if err != nil {
		return Observation{}, err
	}
	template, err := instanceTemplate(a.Config, i)
	if err != nil {
		return Observation{}, err
	}
	_, err = a.formation.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName: aws.String(a.prefix + owner(i)[:24]), TemplateBody: aws.String(template), Capabilities: []cf.Capability{cf.CapabilityCapabilityIam}, ClientRequestToken: aws.String(owner(i)),
		Parameters: []cf.Parameter{{ParameterKey: aws.String("AgentToken"), ParameterValue: aws.String(i.AgentToken)}, {ParameterKey: aws.String("RconPassword"), ParameterValue: aws.String(i.RconPassword)}},
		Tags:       []cf.Tag{{Key: aws.String("insta-game:kind"), Value: aws.String("instance")}, {Key: aws.String("insta-game:owner"), Value: aws.String(owner(i))}, {Key: aws.String("insta-game:environment"), Value: aws.String(a.Config.Stack)}},
	})
	return Observation{Exists: true, Pending: true}, err
}

func (a *AWS) Inspect(ctx context.Context, i Instance) (Observation, error) {
	stack, err := a.stack(ctx, i)
	if cloud.MissingStack(err) {
		current, readErr := a.Store.Get(i.ID)
		if readErr != nil {
			return Observation{}, readErr
		}
		if current.AWS.Game.Generation > 0 {
			return Observation{}, fmt.Errorf("instance resources disappeared after a task was launched")
		}
		return Observation{}, nil
	}
	if err != nil {
		return Observation{}, err
	}
	observation := Observation{Exists: true}
	if stack.StackStatus == cf.StackStatusCreateInProgress {
		observation.Pending = true
		return observation, nil
	}
	if stack.StackStatus != cf.StackStatusCreateComplete {
		return observation, fmt.Errorf("instance stack is %s: %s", stack.StackStatus, aws.ToString(stack.StackStatusReason))
	}
	task, err := a.resolveTask(ctx, i, false)
	if errors.Is(err, ErrPending) {
		observation.Pending = true
		observation.Running = true
		return observation, nil
	}
	if err != nil {
		return observation, err
	}
	current, err := a.Store.Get(i.ID)
	if err != nil {
		return observation, err
	}
	attempt := current.AWS.Game
	observation.Started = attempt.Generation > 0 && attempt.Generation == i.Generation
	if attempt.Result != nil {
		observation.Failure = attempt.Result.Reason
		return observation, nil
	}
	if task == nil {
		return observation, nil
	}
	status := aws.ToString(task.LastStatus)
	observation.Running = status != "STOPPED"
	observation.Pending = status != "RUNNING" && status != "STOPPED"
	if observation.Pending || !observation.Running {
		return observation, nil
	}
	var interfaceID string
	for _, attachment := range task.Attachments {
		for _, detail := range attachment.Details {
			if aws.ToString(detail.Name) == "networkInterfaceId" {
				interfaceID = aws.ToString(detail.Value)
			}
		}
	}
	if interfaceID == "" {
		return observation, fmt.Errorf("running task has no network interface")
	}
	interfaces, err := a.network.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: []string{interfaceID}})
	if err != nil {
		return observation, err
	}
	if len(interfaces.NetworkInterfaces) != 1 || interfaces.NetworkInterfaces[0].Association == nil {
		return observation, fmt.Errorf("task public address is not available")
	}
	address := aws.ToString(interfaces.NetworkInterfaces[0].Association.PublicIp)
	if net.ParseIP(address) == nil {
		return observation, fmt.Errorf("invalid task public address")
	}
	observation.AgentURL = "https://" + net.JoinHostPort(address, strconv.Itoa(i.Game.AgentPort))
	for key := range i.Game.Ports {
		parts := strings.Split(key, "/")
		port, _ := strconv.Atoi(parts[0])
		if port == i.Game.AgentPort && parts[1] == "tcp" {
			continue
		}
		observation.Endpoints = append(observation.Endpoints, Endpoint{Port: port, Protocol: parts[1], Address: net.JoinHostPort(address, strconv.Itoa(port))})
	}
	return observation, nil
}

func (a *AWS) Start(ctx context.Context, i Instance) error {
	err := a.Store.Update(i.ID, func(current *Instance) error {
		if current.Generation != i.Generation || current.Desired != "running" {
			return ErrPending
		}
		attempt := current.AWS.Game
		if attempt.Generation == i.Generation {
			if attempt.Result != nil {
				return fmt.Errorf("attempt stopped; stop and start to retry")
			}
			return nil
		}
		if attempt.Generation > 0 && attempt.Result == nil {
			return ErrPending
		}
		current.AWS.Game = TaskAttempt{Generation: i.Generation, SubmittedAt: time.Now().UTC()}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = a.resolveTask(ctx, i, false)
	return err
}

func (a *AWS) Stop(ctx context.Context, i Instance) error {
	task, err := a.resolveTask(ctx, i, false)
	if err != nil {
		return err
	}
	return a.stopTask(ctx, task)
}

func (a *AWS) stopTask(ctx context.Context, task *ecst.Task) error {
	if task == nil {
		return nil
	}
	status := aws.ToString(task.LastStatus)
	if status == "STOPPED" || status == "STOPPING" || status == "DEPROVISIONING" {
		return nil
	}
	_, err := a.tasks.StopTask(ctx, &ecs.StopTaskInput{Cluster: aws.String(a.Config.Cluster), Task: task.TaskArn, Reason: aws.String("insta-game desired state")})
	return err
}

func (a *AWS) Delete(ctx context.Context, i Instance) error {
	task, err := a.resolveTask(ctx, i, false)
	if err != nil {
		return err
	}
	if task != nil && aws.ToString(task.LastStatus) != "STOPPED" {
		if err := a.stopTask(ctx, task); err != nil {
			return err
		}
		return ErrPending
	}
	current, err := a.Store.Get(i.ID)
	if err != nil {
		return err
	}
	i.AWS = current.AWS
	stack, err := a.stack(ctx, i)
	if cloud.MissingStack(err) {
		if i.AWS.Game.Generation > 0 && !cleanupSucceeded(i.AWS.Cleanup) {
			return fmt.Errorf("instance resources disappeared before data cleanup was verified")
		}
		return nil
	}
	if err != nil {
		return err
	}
	switch stack.StackStatus {
	case cf.StackStatusCreateInProgress, cf.StackStatusDeleteInProgress:
		return ErrPending
	case cf.StackStatusCreateComplete:
		if !cleanupSucceeded(i.AWS.Cleanup) {
			if i.AWS.Cleanup.Result != nil && !i.AWS.Cleanup.Result.Expired && i.AWS.Cleanup.Generation == i.Generation {
				return fmt.Errorf("data cleanup failed; inspect its logs and retry delete")
			}
			if err := a.Store.Update(i.ID, func(current *Instance) error {
				attempt := current.AWS.Cleanup
				if attempt.Generation == 0 || (attempt.Result != nil && (attempt.Result.Expired || attempt.Generation != current.Generation)) {
					current.AWS.Cleanup = TaskAttempt{Generation: current.Generation, Number: attempt.Number + 1, SubmittedAt: time.Now().UTC()}
				}
				return nil
			}); err != nil {
				return err
			}
			if _, err := a.resolveTask(ctx, i, true); err != nil {
				return err
			}
			return ErrPending
		}
	case cf.StackStatusRollbackComplete, cf.StackStatusCreateFailed, cf.StackStatusDeleteFailed:
		if (i.AWS.Game.Generation > 0 || i.AWS.Cleanup.Generation > 0) && !cleanupSucceeded(i.AWS.Cleanup) {
			return fmt.Errorf("instance stack is %s before data cleanup was verified", stack.StackStatus)
		}
	default:
		return fmt.Errorf("cannot delete instance stack in state %s", stack.StackStatus)
	}
	if _, err := a.formation.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: stack.StackId}); err != nil {
		return err
	}
	return ErrPending
}

func cleanupSucceeded(attempt TaskAttempt) bool {
	return attempt.Result != nil && attempt.Result.ExitCode != nil && *attempt.Result.ExitCode == 0
}

var _ Provider = (*AWS)(nil)
