package fleet

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
)

type EC2 struct {
	Client *ec2.Client
	Stack  string
}

func (e *EC2) find(ctx context.Context, h Host) (*types.Instance, error) {
	input := &ec2.DescribeInstancesInput{}
	if h.InstanceID != "" {
		input.InstanceIds = []string{h.InstanceID}
	} else {
		input.Filters = []types.Filter{{Name: aws.String("tag:insta-game:fleet"), Values: []string{e.Stack}}, {Name: aws.String("tag:insta-game:host"), Values: []string{h.ID}}, {Name: aws.String("client-token"), Values: []string{h.LaunchToken}}}
	}
	out, err := e.Client.DescribeInstances(ctx, input)
	if err != nil {
		var api smithy.APIError
		if h.InstanceID != "" && errors.As(err, &api) && api.ErrorCode() == "InvalidInstanceID.NotFound" {
			return nil, nil
		}
		return nil, err
	}
	var found *types.Instance
	for _, r := range out.Reservations {
		for _, v := range r.Instances {
			tags := map[string]string{}
			for _, t := range v.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
			if tags["insta-game:fleet"] != e.Stack || tags["insta-game:host"] != h.ID || aws.ToString(v.ClientToken) != h.LaunchToken {
				return nil, fmt.Errorf("refusing unmanaged host %s", h.ID)
			}
			if found != nil {
				return nil, fmt.Errorf("multiple machines for host %s", h.ID)
			}
			copy := v
			found = &copy
		}
	}
	return found, nil
}
func (e *EC2) Inspect(ctx context.Context, h Host) (Machine, error) {
	v, err := e.find(ctx, h)
	if err != nil {
		return Machine{}, err
	}
	if v == nil {
		if h.InstanceID != "" {
			return Machine{ID: h.InstanceID, State: "terminated"}, nil
		}
		return Machine{State: "new"}, nil
	}
	if v.State == nil {
		return Machine{}, fmt.Errorf("host has no state")
	}
	return Machine{ID: aws.ToString(v.InstanceId), State: string(v.State.Name)}, nil
}
func (e *EC2) Wake(ctx context.Context, h Host) (string, error) {
	if !validID.MatchString(h.ID) || h.LaunchToken == "" || len(h.LaunchToken) > 64 || !strings.HasPrefix(h.LaunchTemplate, "lt-") || !launchVersion.MatchString(h.LaunchVersion) {
		return "", fmt.Errorf("invalid saved host launch identity")
	}

	v, err := e.find(ctx, h)
	if err != nil {
		return "", err
	}
	if v == nil {
		if h.InstanceID != "" {
			return "", ErrUnavailable
		}
		out, err := e.Client.RunInstances(ctx, &ec2.RunInstancesInput{MinCount: aws.Int32(1), MaxCount: aws.Int32(1), ClientToken: aws.String(h.LaunchToken), LaunchTemplate: &types.LaunchTemplateSpecification{LaunchTemplateId: aws.String(h.LaunchTemplate), Version: aws.String(h.LaunchVersion)}, TagSpecifications: []types.TagSpecification{{ResourceType: types.ResourceTypeInstance, Tags: []types.Tag{{Key: aws.String("insta-game:fleet"), Value: aws.String(e.Stack)}, {Key: aws.String("insta-game:host"), Value: aws.String(h.ID)}, {Key: aws.String("Name"), Value: aws.String(h.ID)}}}, {ResourceType: types.ResourceTypeVolume, Tags: []types.Tag{{Key: aws.String("insta-game:fleet"), Value: aws.String(e.Stack)}, {Key: aws.String("insta-game:host"), Value: aws.String(h.ID)}}}}})
		if err != nil {
			return "", err
		}
		if len(out.Instances) != 1 {
			return "", fmt.Errorf("expected one launched host")
		}
		return aws.ToString(out.Instances[0].InstanceId), nil
	}
	if v.State == nil {
		return "", fmt.Errorf("host has no state")
	}
	id := aws.ToString(v.InstanceId)
	switch v.State.Name {
	case types.InstanceStateNameStopped:
		_, err = e.Client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{id}})
	case types.InstanceStateNameRunning, types.InstanceStateNamePending, types.InstanceStateNameStopping:
	default:
		err = fmt.Errorf("host %s is %s; its data has not been reassigned", h.ID, v.State.Name)
	}
	return id, err
}
func (e *EC2) Delete(ctx context.Context, h Host) error {
	v, err := e.find(ctx, h)
	if err != nil {
		return err
	}
	// Recover a timed-out RunInstances before retiring its slot, even if its instance ID was never saved.
	if v == nil && h.InstanceID != "" {
		return e.disksDeleted(ctx, h)
	}
	if v == nil {
		if _, err = e.Wake(ctx, h); err != nil {
			return err
		}
		v, err = e.find(ctx, h)
		if err != nil {
			return err
		}
		if v == nil {
			return fmt.Errorf("waiting for host visibility")
		}
	}
	if v.State == nil {
		return fmt.Errorf("host has no state")
	}
	if v.State.Name == types.InstanceStateNameTerminated {
		return e.disksDeleted(ctx, h)
	}
	if v.State.Name != types.InstanceStateNameShuttingDown {
		_, err = e.Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{aws.ToString(v.InstanceId)}})
		if err != nil {
			return err
		}
	}
	return fmt.Errorf("waiting for %s to terminate", h.ID)
}

func (e *EC2) Stop(ctx context.Context, h Host) error {
	v, err := e.find(ctx, h)
	if err != nil || v == nil {
		return err
	}
	if v.State == nil {
		return fmt.Errorf("host has no state")
	}
	switch v.State.Name {
	case types.InstanceStateNameStopped, types.InstanceStateNameStopping, types.InstanceStateNameTerminated, types.InstanceStateNameShuttingDown:
		return nil
	case types.InstanceStateNameRunning, types.InstanceStateNamePending:
		_, err = e.Client.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{aws.ToString(v.InstanceId)}})
		return err
	}
	return fmt.Errorf("cannot stop host")
}

func (e *EC2) disksDeleted(ctx context.Context, h Host) error {
	out, err := e.Client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{Filters: []types.Filter{{Name: aws.String("tag:insta-game:fleet"), Values: []string{e.Stack}}, {Name: aws.String("tag:insta-game:host"), Values: []string{h.ID}}}})
	if err != nil {
		return err
	}
	if len(out.Volumes) > 0 {
		return fmt.Errorf("waiting for host root disk deletion")
	}
	return nil
}
