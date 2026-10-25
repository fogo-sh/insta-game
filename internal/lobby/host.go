package lobby

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Host struct {
	Client              *ec2.Client
	InstanceID, StackID string
}

func (h *Host) State(ctx context.Context) (string, error) {
	result, err := h.Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{h.InstanceID}})
	if err != nil {
		return "", err
	}
	if len(result.Reservations) != 1 || len(result.Reservations[0].Instances) != 1 {
		return "", fmt.Errorf("expected one owned game host")
	}
	instance := result.Reservations[0].Instances[0]
	owned := false
	for _, tag := range instance.Tags {
		if aws.ToString(tag.Key) == "aws:cloudformation:stack-id" && aws.ToString(tag.Value) == h.StackID {
			owned = true
		}
	}
	if !owned || instance.State == nil {
		return "", fmt.Errorf("refusing unmanaged game host")
	}
	return string(instance.State.Name), nil
}
func (h *Host) Wake(ctx context.Context) error {
	state, err := h.State(ctx)
	if err != nil {
		return err
	}
	switch types.InstanceStateName(state) {
	case types.InstanceStateNameStopped:
		_, err = h.Client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{h.InstanceID}})
		return err
	case types.InstanceStateNameRunning, types.InstanceStateNamePending, types.InstanceStateNameStopping:
		return nil // The scheduled retry handles a host already committed to shutdown.
	default:
		return fmt.Errorf("cannot wake game host in state %s", state)
	}
}
