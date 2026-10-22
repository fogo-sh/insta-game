package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"golang.org/x/crypto/ssh"
)

type HostState struct {
	InstanceID string
	State      string
	PublicIP   string
}

// Host controls one existing, stack-owned EC2 machine. Games stay on its controller.
type Host struct {
	Config Config
	ec2    *ec2.Client
	stacks *cloudformation.Client
	mu     sync.Mutex
}

func NewHost(ctx context.Context, c Config) (*Host, error) {
	sdk, err := LoadSDK(ctx, c.Region)
	if err != nil {
		return nil, err
	}
	h := &Host{Config: c, ec2: ec2.NewFromConfig(sdk), stacks: cloudformation.NewFromConfig(sdk)}
	if err := h.verifyOwnership(ctx); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Host) verifyOwnership(ctx context.Context) error {
	stack, err := DescribeStack(ctx, h.stacks, h.Config.Stack)
	if err != nil {
		return err
	}
	if h.Config.InstanceID == "" || Outputs(stack)["InstanceID"] != h.Config.InstanceID {
		return fmt.Errorf("instance does not belong to the configured shared-host stack")
	}
	return nil
}

func (h *Host) Inspect(ctx context.Context) (HostState, error) {
	result, err := h.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{h.Config.InstanceID}})
	if err != nil {
		return HostState{}, err
	}
	if len(result.Reservations) != 1 || len(result.Reservations[0].Instances) != 1 {
		return HostState{}, fmt.Errorf("expected one EC2 host")
	}
	instance := result.Reservations[0].Instances[0]
	if instance.State == nil {
		return HostState{}, fmt.Errorf("EC2 host has no state")
	}
	return HostState{InstanceID: aws.ToString(instance.InstanceId), State: string(instance.State.Name), PublicIP: aws.ToString(instance.PublicIpAddress)}, nil
}

func (h *Host) Wake(ctx context.Context) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	if err := h.verifyOwnership(ctx); err != nil {
		return "", err
	}
	for {
		state, err := h.Inspect(ctx)
		if err != nil {
			return "", err
		}
		switch state.State {
		case "running":
			if state.PublicIP != "" {
				return state.PublicIP, nil
			}
		case "stopped":
			if _, err := h.ec2.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{h.Config.InstanceID}}); err != nil {
				return "", err
			}
		case "pending", "stopping":
		default:
			return "", fmt.Errorf("cannot wake EC2 host in state %s", state.State)
		}
		if err := waitPoll(ctx); err != nil {
			return "", err
		}
	}
}

func (h *Host) Stop(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.verifyOwnership(ctx); err != nil {
		return err
	}
	state, err := h.Inspect(ctx)
	if err != nil || state.State == "stopped" {
		return err
	}
	if state.State != "stopping" {
		if _, err := h.ec2.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{h.Config.InstanceID}}); err != nil {
			return err
		}
	}
	return ec2.NewInstanceStoppedWaiter(h.ec2).Wait(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{h.Config.InstanceID}}, 5*time.Minute)
}

// ConsoleHostKey anchors SSH trust in the authenticated EC2 API before sending credentials.
func (h *Host) ConsoleHostKey(ctx context.Context) (ssh.PublicKey, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	for {
		result, err := h.ec2.GetConsoleOutput(ctx, &ec2.GetConsoleOutputInput{InstanceId: aws.String(h.Config.InstanceID), Latest: aws.Bool(true)})
		if err != nil {
			return nil, err
		}
		if aws.ToString(result.InstanceId) != h.Config.InstanceID {
			return nil, fmt.Errorf("console response belongs to another instance")
		}
		if result.Output != nil {
			decoded, err := base64.StdEncoding.DecodeString(*result.Output)
			if err != nil {
				return nil, fmt.Errorf("decode EC2 console: %w", err)
			}
			key, err := consoleHostKey(string(decoded))
			if err != nil || key != nil {
				return key, err
			}
		}
		if err := waitPoll(ctx); err != nil {
			return nil, fmt.Errorf("waiting for authenticated SSH host key: %w", err)
		}
	}
}

func consoleHostKey(output string) (ssh.PublicKey, error) {
	var found ssh.PublicKey
	for _, line := range strings.Split(output, "\n") {
		material, ok := strings.CutPrefix(strings.TrimSpace(line), "INSTA_GAME_SSH_HOST_KEY=")
		if !ok {
			continue
		}
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(material))
		if err != nil || key.Type() != ssh.KeyAlgoED25519 {
			return nil, fmt.Errorf("invalid SSH host key in EC2 console")
		}
		if found != nil && string(found.Marshal()) != string(key.Marshal()) {
			return nil, fmt.Errorf("conflicting SSH host keys in EC2 console")
		}
		found = key
	}
	return found, nil
}

func waitPoll(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
		return nil
	}
}
