package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecst "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"
	"github.com/fogo-sh/insta-game/internal/cloud"
)

func TestPendingInfrastructureIsNotTreatedAsCrash(t *testing.T) {
	c, p, _ := setup(t)
	i := create(t, c, "pending")
	if err := c.Store.Update(i.ID, func(current *Instance) error { current.Phase = "starting"; return nil }); err != nil {
		t.Fatal(err)
	}
	p.resources[i.ID] = Observation{Exists: true, Started: true, Pending: true}
	c.Reconcile(context.Background())
	observed, err := c.Store.Get(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Phase != "starting" || observed.Error != "" || p.resources[i.ID].Running {
		t.Fatalf("pending task was failed or started twice: %+v", observed)
	}
	p.resources[i.ID] = Observation{Exists: true, Started: true}
	c.Reconcile(context.Background())
	observed, err = c.Store.Get(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if observed.Phase != "failed" {
		t.Fatalf("stopped attempt was not reported failed: %s", observed.Phase)
	}
}

func TestAWSReferencesPersistPrivately(t *testing.T) {
	c, _, _ := setup(t)
	i := create(t, c, "private-state")
	state := AWSState{Environment: "environment-arn", Game: TaskAttempt{Generation: 3, ARN: "task-arn"}, Cleanup: TaskAttempt{Generation: 1, ARN: "cleanup-arn"}, DataURL: "private-assets"}
	if err := c.Store.Update(i.ID, func(current *Instance) error { current.AWS = state; return nil }); err != nil {
		t.Fatal(err)
	}
	stored, err := c.Store.Get(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AWS != state {
		t.Fatalf("provider references were lost: %+v", stored.AWS)
	}
	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]json.RawMessage
	if err := json.Unmarshal(data, &public); err != nil {
		t.Fatal(err)
	}
	if _, ok := public["AWS"]; ok {
		t.Fatal("provider references entered the API response")
	}
}

type fakeFormation struct {
	formationAPI
	stack   cf.Stack
	deleted bool
	creates int
}

func (f *fakeFormation) CreateStack(context.Context, *cloudformation.CreateStackInput, ...func(*cloudformation.Options)) (*cloudformation.CreateStackOutput, error) {
	f.creates++
	f.deleted = false
	f.stack.StackStatus = cf.StackStatusCreateInProgress
	return &cloudformation.CreateStackOutput{StackId: f.stack.StackId}, nil
}

func (f *fakeFormation) DescribeStacks(context.Context, *cloudformation.DescribeStacksInput, ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	if f.deleted {
		return nil, &smithy.GenericAPIError{Code: "ValidationError", Message: "stack does not exist"}
	}
	return &cloudformation.DescribeStacksOutput{Stacks: []cf.Stack{f.stack}}, nil
}

func (f *fakeFormation) DeleteStack(context.Context, *cloudformation.DeleteStackInput, ...func(*cloudformation.Options)) (*cloudformation.DeleteStackOutput, error) {
	f.deleted = true
	return &cloudformation.DeleteStackOutput{}, nil
}

type fakeTasks struct {
	tasksAPI
	tasks               map[string]ecst.Task
	tokens              map[string]string
	runs                []*ecs.RunTaskInput
	stops               []string
	loseReply, hideList bool
	describeErr         error
	beforeRun           func(*ecs.RunTaskInput)
}

func (f *fakeTasks) DescribeTasks(_ context.Context, input *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	if task, ok := f.tasks[input.Tasks[0]]; ok {
		return &ecs.DescribeTasksOutput{Tasks: []ecst.Task{task}}, nil
	}
	return &ecs.DescribeTasksOutput{Failures: []ecst.Failure{{Arn: aws.String(input.Tasks[0]), Reason: aws.String("MISSING")}}}, nil
}

func (f *fakeTasks) ListTasks(_ context.Context, input *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	result := &ecs.ListTasksOutput{}
	for id, task := range f.tasks {
		if !f.hideList && aws.ToString(task.StartedBy) == aws.ToString(input.StartedBy) && aws.ToString(task.LastStatus) != "STOPPED" {
			result.TaskArns = append(result.TaskArns, id)
		}
	}
	return result, nil
}

func (f *fakeTasks) RunTask(_ context.Context, input *ecs.RunTaskInput, _ ...func(*ecs.Options)) (*ecs.RunTaskOutput, error) {
	if f.beforeRun != nil {
		f.beforeRun(input)
	}
	f.runs = append(f.runs, input)
	token := aws.ToString(input.ClientToken)
	id, exists := f.tokens[token]
	if !exists {
		id = fmt.Sprintf("task-%d", len(f.tokens)+1)
		f.tokens[token] = id
		f.tasks[id] = ecst.Task{TaskArn: aws.String(id), StartedBy: input.StartedBy, Tags: input.Tags, LastStatus: aws.String("PENDING")}
	}
	if f.loseReply {
		f.loseReply = false
		return nil, errors.New("connection lost after accepting task")
	}
	return &ecs.RunTaskOutput{Tasks: []ecst.Task{f.tasks[id]}}, nil
}

func (f *fakeTasks) StopTask(_ context.Context, input *ecs.StopTaskInput, _ ...func(*ecs.Options)) (*ecs.StopTaskOutput, error) {
	id := aws.ToString(input.Task)
	f.stops = append(f.stops, id)
	f.finish(id, 0)
	return &ecs.StopTaskOutput{}, nil
}

func (f *fakeTasks) finish(id string, code int32) {
	task := f.tasks[id]
	task.LastStatus = aws.String("STOPPED")
	task.Containers = []ecst.Container{{Name: aws.String("game"), ExitCode: aws.Int32(code)}}
	f.tasks[id] = task
}

func awsFixture(t *testing.T) (*Controller, *AWS, *fakeTasks, *fakeFormation, Instance) {
	t.Helper()
	c, _, _ := setup(t)
	i := create(t, c, "aws-server")
	f := &fakeFormation{stack: cf.Stack{StackId: aws.String("instance-stack"), StackStatus: cf.StackStatusCreateComplete,
		Tags:    []cf.Tag{{Key: aws.String("insta-game:kind"), Value: aws.String("instance")}, {Key: aws.String("insta-game:owner"), Value: aws.String(owner(i))}},
		Outputs: []cf.Output{{OutputKey: aws.String("Definition"), OutputValue: aws.String("task-definition")}, {OutputKey: aws.String("SecurityGroup"), OutputValue: aws.String("game-group")}},
	}}
	tasks := &fakeTasks{tasks: map[string]ecst.Task{}, tokens: map[string]string{}}
	a := &AWS{Store: c.Store, Config: cloud.Config{Stack: "environment", Cluster: "cluster"}, tasks: tasks, formation: f}
	c.Providers["aws"] = a
	if err := c.Store.Update(i.ID, func(current *Instance) error {
		current.Provider = "aws"
		current.AWS.Environment = a.Config.Stack
		current.Game.DataRoot = "/opt"
		current.Game.AgentPort = 5001
		current.Game.ConfigPath = "/opt/data/server.cfg"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	i, err := c.Store.Get(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	return c, a, tasks, f, i
}

func TestAWSLostLaunchResponseIsRecoveredBeforeDelete(t *testing.T) {
	for _, hideList := range []bool{false, true} {
		t.Run(fmt.Sprintf("replay=%v", hideList), func(t *testing.T) {
			c, a, tasks, formation, i := awsFixture(t)
			tasks.loseReply, tasks.hideList = true, hideList
			tasks.beforeRun = func(input *ecs.RunTaskInput) {
				stored, err := c.Store.Get(i.ID)
				if err != nil || stored.AWS.Game.Generation != i.Generation || stored.AWS.Game.SubmittedAt.IsZero() {
					t.Fatalf("submission was not checkpointed: %+v %v", stored.AWS.Game, err)
				}
			}
			if err := a.Start(context.Background(), i); err == nil {
				t.Fatal("lost response reported success")
			}
			if err := c.SetDesired(i.ID, "deleted"); err != nil {
				t.Fatal(err)
			}
			tasks.beforeRun = nil
			c.Reconcile(context.Background())
			stored, _ := c.Store.Get(i.ID)
			if len(tasks.stops) != 1 || stored.AWS.Game.ARN != tasks.stops[0] || stored.AWS.Cleanup.Generation != 0 || formation.deleted {
				t.Fatalf("deletion skipped accepted game: %+v, stops=%v", stored.AWS, tasks.stops)
			}
			if len(tasks.tasks) != 1 {
				t.Fatal("recovery created a second game")
			}
			if hideList && (len(tasks.runs) != 2 || aws.ToString(tasks.runs[0].ClientToken) != aws.ToString(tasks.runs[1].ClientToken)) {
				t.Fatal("replay changed the attempt token after intent changed")
			}
			c.Reconcile(context.Background())
			stored, _ = c.Store.Get(i.ID)
			if stored.AWS.Cleanup.ARN == "" || formation.deleted {
				t.Fatal("cleanup was not scheduled before stack deletion")
			}
			tasks.finish(stored.AWS.Cleanup.ARN, 0)
			for range 3 {
				c.Reconcile(context.Background())
			}
			stored, _ = c.Store.Get(i.ID)
			if stored.Phase != "deleted" || !formation.deleted || !cleanupSucceeded(stored.AWS.Cleanup) {
				t.Fatalf("cleanup/deletion did not finish: %+v", stored)
			}
		})
	}
}

func TestAWSExpiredGameHistoryFailsWithoutRelaunch(t *testing.T) {
	c, _, tasks, _, i := awsFixture(t)
	if err := c.Store.Update(i.ID, func(current *Instance) error {
		current.Phase = "running"
		current.AWS.Game = TaskAttempt{Generation: i.Generation, ARN: "forgotten-task", SubmittedAt: time.Now().Add(-2 * time.Hour)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	c.Reconcile(context.Background())
	stored, _ := c.Store.Get(i.ID)
	if stored.Phase != "failed" || stored.AWS.Game.Result == nil || !stored.AWS.Game.Result.Expired || len(tasks.runs) != 0 {
		t.Fatalf("expired history caused a relaunch or stalled: %+v", stored)
	}
}

func TestAWSFailedCleanupRequiresExplicitRetryAndNewAttempt(t *testing.T) {
	c, _, tasks, formation, i := awsFixture(t)
	if err := c.SetDesired(i.ID, "deleted"); err != nil {
		t.Fatal(err)
	}
	c.Reconcile(context.Background())
	stored, _ := c.Store.Get(i.ID)
	first := stored.AWS.Cleanup.ARN
	tasks.finish(first, 1)
	c.Reconcile(context.Background())
	c.Reconcile(context.Background())
	stored, _ = c.Store.Get(i.ID)
	if stored.Error == "" || len(tasks.runs) != 1 || formation.deleted {
		t.Fatal("failed cleanup was ignored or automatically retried")
	}
	if err := c.SetDesired(i.ID, "deleted"); err != nil {
		t.Fatal(err)
	}
	c.Reconcile(context.Background())
	stored, _ = c.Store.Get(i.ID)
	if stored.AWS.Cleanup.ARN == first || len(tasks.runs) != 2 || aws.ToString(tasks.runs[0].ClientToken) == aws.ToString(tasks.runs[1].ClientToken) {
		t.Fatal("explicit retry reused failed cleanup task")
	}
	tasks.finish(stored.AWS.Cleanup.ARN, 0)
	for range 3 {
		c.Reconcile(context.Background())
	}
	stored, _ = c.Store.Get(i.ID)
	if stored.Phase != "deleted" || !formation.deleted {
		t.Fatalf("retried cleanup did not finish: %+v", stored)
	}
}

func TestAWSExpiredCleanupIsRerunBeforeDeletingStack(t *testing.T) {
	c, _, tasks, formation, i := awsFixture(t)
	if err := c.SetDesired(i.ID, "deleted"); err != nil {
		t.Fatal(err)
	}
	if err := c.Store.Update(i.ID, func(current *Instance) error {
		current.AWS.Cleanup = TaskAttempt{Generation: current.Generation, Number: 1, ARN: "forgotten-cleanup", SubmittedAt: time.Now().Add(-2 * time.Hour)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	c.Reconcile(context.Background())
	c.Reconcile(context.Background())
	stored, _ := c.Store.Get(i.ID)
	if formation.deleted || len(tasks.runs) != 1 || stored.AWS.Cleanup.Number != 2 || stored.AWS.Cleanup.ARN == "forgotten-cleanup" {
		t.Fatalf("unverified cleanup skipped or stalled: %+v", stored.AWS.Cleanup)
	}
}

func TestAWSTaskErrorsAndOwnershipNeverBecomeAbsent(t *testing.T) {
	for _, mode := range []string{"request failure", "wrong owner"} {
		t.Run(mode, func(t *testing.T) {
			c, a, tasks, formation, i := awsFixture(t)
			if err := a.Start(context.Background(), i); err != nil {
				t.Fatal(err)
			}
			stored, _ := c.Store.Get(i.ID)
			if mode == "request failure" {
				tasks.describeErr = errors.New("access denied")
			} else {
				task := tasks.tasks[stored.AWS.Game.ARN]
				task.Tags = nil
				tasks.tasks[stored.AWS.Game.ARN] = task
			}
			if err := c.SetDesired(i.ID, "deleted"); err != nil {
				t.Fatal(err)
			}
			c.Reconcile(context.Background())
			stored, _ = c.Store.Get(i.ID)
			if stored.Error == "" || stored.AWS.Game.Result != nil || formation.deleted || len(tasks.stops) != 0 || len(tasks.runs) != 1 {
				t.Fatalf("unsafe recovery after %s: %+v", mode, stored)
			}
		})
	}
}

func TestAWSMissingStackDoesNotReplaceLaunchedInstance(t *testing.T) {
	c, a, _, formation, i := awsFixture(t)
	if err := a.Start(context.Background(), i); err != nil {
		t.Fatal(err)
	}
	formation.deleted = true
	c.Reconcile(context.Background())
	stored, _ := c.Store.Get(i.ID)
	if stored.Error == "" || formation.creates != 0 {
		t.Fatal("missing infrastructure silently replaced a launched instance")
	}
}

func TestAWSFailedStackDeletionCannotBypassDataCleanup(t *testing.T) {
	for _, stopped := range []bool{false, true} {
		t.Run(fmt.Sprintf("stopped=%v", stopped), func(t *testing.T) {
			c, _, tasks, formation, i := awsFixture(t)
			if err := c.Store.Update(i.ID, func(current *Instance) error {
				current.Desired = "deleted"
				current.AWS.Game = TaskAttempt{Generation: i.Generation, SubmittedAt: time.Now()}
				if stopped {
					current.AWS.Game.Result = &TaskResult{ExitCode: aws.Int32(0)}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			formation.stack.StackStatus = cf.StackStatusDeleteFailed
			c.Reconcile(context.Background())
			stored, _ := c.Store.Get(i.ID)
			if stored.Error == "" || formation.deleted || len(tasks.runs) != 0 {
				t.Fatal("failed stack was reused or deleted before data cleanup")
			}
		})
	}
}
