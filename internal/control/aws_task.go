package control

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecst "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fogo-sh/insta-game/internal/cloud"
)

type formationAPI interface {
	cloud.StackDescriber
	CreateStack(context.Context, *cloudformation.CreateStackInput, ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error)
	DeleteStack(context.Context, *cloudformation.DeleteStackInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error)
}

type tasksAPI interface {
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	RunTask(context.Context, *ecs.RunTaskInput, ...func(*ecs.Options)) (*ecs.RunTaskOutput, error)
	StopTask(context.Context, *ecs.StopTaskInput, ...func(*ecs.Options)) (*ecs.StopTaskOutput, error)
}

type networkAPI interface {
	DescribeNetworkInterfaces(context.Context, *ec2.DescribeNetworkInterfacesInput, ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
}

var errTaskMissing = errors.New("task history is missing")

func (a *AWS) task(ctx context.Context, i Instance, id string) (*ecst.Task, error) {
	result, err := a.tasks.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: aws.String(a.Config.Cluster), Tasks: []string{id}, Include: []ecst.TaskField{ecst.TaskFieldTags}})
	if err != nil {
		return nil, err
	}
	if len(result.Tasks) != 1 {
		if len(result.Failures) == 1 && aws.ToString(result.Failures[0].Reason) == "MISSING" {
			return nil, errTaskMissing
		}
		return nil, fmt.Errorf("AWS task is not observable: %v", result.Failures)
	}
	task := &result.Tasks[0]
	for _, tag := range task.Tags {
		if aws.ToString(tag.Key) == "insta-game:owner" && aws.ToString(tag.Value) == owner(i) {
			return task, nil
		}
	}
	return nil, fmt.Errorf("refusing an unowned AWS task")
}

func attemptToken(i Instance, attempt TaskAttempt, cleanup bool) string {
	kind := "game"
	if cleanup {
		kind = "cleanup"
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d:%d", owner(i), kind, attempt.Generation, attempt.Number)))
	return fmt.Sprintf("%x", digest)
}

func (a *AWS) findAttempt(ctx context.Context, i Instance, attempt TaskAttempt, cleanup bool) (*ecst.Task, error) {
	token := attemptToken(i, attempt, cleanup)
	var found *ecst.Task
	var next *string
	for {
		result, err := a.tasks.ListTasks(ctx, &ecs.ListTasksInput{Cluster: aws.String(a.Config.Cluster), StartedBy: aws.String("ig-" + token[:32]), NextToken: next})
		if err != nil {
			return nil, err
		}
		for _, id := range result.TaskArns {
			task, err := a.task(ctx, i, id)
			if errors.Is(err, errTaskMissing) {
				return nil, ErrPending
			}
			if err != nil {
				return nil, err
			}
			if found != nil {
				return nil, fmt.Errorf("multiple tasks found for one instance attempt")
			}
			found = task
		}
		next = result.NextToken
		if next == nil || *next == "" {
			break
		}
	}
	return found, nil
}

func (a *AWS) saveAttempt(i Instance, attempt TaskAttempt, cleanup bool, task *ecst.Task, result *TaskResult) error {
	return a.Store.Update(i.ID, func(current *Instance) error {
		destination := &current.AWS.Game
		if cleanup {
			destination = &current.AWS.Cleanup
		}
		if destination.Generation != attempt.Generation || destination.Number != attempt.Number {
			return ErrPending
		}
		if destination.Result != nil {
			return nil
		}
		if task != nil {
			id := aws.ToString(task.TaskArn)
			if id == "" {
				return fmt.Errorf("AWS returned an empty task ARN")
			}
			if destination.ARN != "" && destination.ARN != id {
				return fmt.Errorf("attempt resolved to multiple task ARNs")
			}
			destination.ARN = id
		}
		destination.Result = result
		return nil
	})
}

// A persisted attempt must be resolved even after intent changes to stop/delete.
// Recent unknown submissions replay the same ECS idempotency token. Expired
// history becomes terminal only after a complete live-attempt search succeeds.
func (a *AWS) resolveTask(ctx context.Context, i Instance, cleanup bool) (*ecst.Task, error) {
	current, err := a.Store.Get(i.ID)
	if err != nil {
		return nil, err
	}
	i.AWS = current.AWS
	if i.AWS.Environment != "" && i.AWS.Environment != a.Config.Stack {
		return nil, fmt.Errorf("instance belongs to another AWS environment")
	}
	attempt := i.AWS.Game
	if cleanup {
		attempt = i.AWS.Cleanup
	}
	if attempt.Generation == 0 || attempt.Result != nil {
		return nil, nil
	}
	if attempt.SubmittedAt.IsZero() {
		return nil, fmt.Errorf("task attempt has no submission time")
	}
	var task *ecst.Task
	if attempt.ARN != "" {
		task, err = a.task(ctx, i, attempt.ARN)
		if err != nil && !errors.Is(err, errTaskMissing) {
			return nil, err
		}
	}
	if task == nil {
		task, err = a.findAttempt(ctx, i, attempt, cleanup)
		if err != nil {
			return nil, err
		}
		if task == nil {
			if time.Since(attempt.SubmittedAt) < time.Hour {
				if attempt.ARN != "" {
					return nil, ErrPending
				}
				task, err = a.submit(ctx, i, attempt, cleanup)
				if err != nil {
					return nil, err
				}
			} else {
				result := &TaskResult{Expired: true, Reason: "Task history expired; no live attempt remains"}
				return nil, a.saveAttempt(i, attempt, cleanup, nil, result)
			}
		}
	}
	var result *TaskResult
	if aws.ToString(task.LastStatus) == "STOPPED" {
		result = &TaskResult{Reason: aws.ToString(task.StoppedReason)}
		for _, container := range task.Containers {
			if aws.ToString(container.Name) == "game" {
				result.ExitCode = container.ExitCode
			}
		}
	}
	if err := a.saveAttempt(i, attempt, cleanup, task, result); err != nil {
		return nil, err
	}
	return task, nil
}

func (a *AWS) submit(ctx context.Context, i Instance, attempt TaskAttempt, cleanup bool) (*ecst.Task, error) {
	stack, err := a.stack(ctx, i)
	if err != nil {
		return nil, err
	}
	if stack.StackStatus != cf.StackStatusCreateComplete {
		return nil, fmt.Errorf("instance stack is %s", stack.StackStatus)
	}
	outputs := cloud.Outputs(stack)
	if outputs["Definition"] == "" || outputs["SecurityGroup"] == "" {
		return nil, fmt.Errorf("instance stack is missing task outputs")
	}
	token := attemptToken(i, attempt, cleanup)
	input := &ecs.RunTaskInput{
		Cluster: aws.String(a.Config.Cluster), TaskDefinition: aws.String(outputs["Definition"]), Count: aws.Int32(1), LaunchType: ecst.LaunchTypeFargate, PlatformVersion: aws.String("1.4.0"), ClientToken: aws.String(token), StartedBy: aws.String("ig-" + token[:32]),
		NetworkConfiguration: &ecst.NetworkConfiguration{AwsvpcConfiguration: &ecst.AwsVpcConfiguration{Subnets: []string{a.Config.Subnet}, SecurityGroups: []string{a.Config.SecurityGroup, outputs["SecurityGroup"]}, AssignPublicIp: ecst.AssignPublicIpEnabled}},
		Tags:                 []ecst.Tag{{Key: aws.String("insta-game:owner"), Value: aws.String(owner(i))}, {Key: aws.String("insta-game:environment"), Value: aws.String(a.Config.Stack)}},
	}
	if cleanup {
		input.Overrides = &ecst.TaskOverride{ContainerOverrides: []ecst.ContainerOverride{{Name: aws.String("game"), Command: []string{"clean-data", "--dir", i.Game.DataRoot, "--owner", owner(i)}}}}
	}
	result, err := a.tasks.RunTask(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(result.Tasks) != 1 {
		return nil, fmt.Errorf("AWS task placement failed: %v", result.Failures)
	}
	if aws.ToString(result.Tasks[0].TaskArn) == "" {
		return nil, fmt.Errorf("AWS returned an empty task ARN")
	}
	return &result.Tasks[0], nil
}
