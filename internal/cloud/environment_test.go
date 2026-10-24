package cloud

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestCloudFormationTemplates(t *testing.T) {
	if os.Getenv("INSTA_GAME_LINT_TEMPLATES") != "1" {
		t.Skip("run make lint-templates for the CloudFormation schema check")
	}
	path := filepath.Join(t.TempDir(), "host.json")
	if err := os.WriteFile(path, []byte(EnvironmentTemplate), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, "cfn-lint", "--template", path).CombinedOutput()
	if err != nil {
		t.Fatalf("CloudFormation template does not validate: %v\n%s", err, output)
	}
}

func TestSharedHostCostAndAccessBoundaries(t *testing.T) {
	var template struct {
		Resources map[string]struct {
			Type       string
			Properties map[string]json.RawMessage
		}
	}
	if err := json.Unmarshal([]byte(EnvironmentTemplate), &template); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"AWS::EC2::VPC": true, "AWS::EC2::InternetGateway": true, "AWS::EC2::VPCGatewayAttachment": true,
		"AWS::EC2::Subnet": true, "AWS::EC2::RouteTable": true, "AWS::EC2::Route": true,
		"AWS::EC2::SubnetRouteTableAssociation": true, "AWS::EC2::SecurityGroup": true,
		"AWS::EC2::KeyPair": true, "AWS::EC2::Instance": true,
		"AWS::DynamoDB::Table": true, "AWS::S3::Bucket": true, "AWS::IAM::Role": true, "AWS::IAM::InstanceProfile": true,
		"AWS::Logs::LogGroup": true, "AWS::Lambda::Function": true, "AWS::Lambda::Url": true, "AWS::Lambda::Permission": true, "AWS::Events::Rule": true,
	}
	hosts := 0
	for id, resource := range template.Resources {
		if !allowed[resource.Type] {
			t.Errorf("unexpected baseline resource %s: %s", id, resource.Type)
		}
		if resource.Type == "AWS::EC2::Instance" {
			hosts++
			if !strings.Contains(string(resource.Properties["IamInstanceProfile"]), `"HasLobby"`) {
				t.Error("host credentials must be conditional on the AWS lobby")
			}
			if !strings.Contains(string(resource.Properties["CreditSpecification"]), `"standard"`) {
				t.Error("surplus CPU billing must be disabled")
			}
			var disks []struct {
				Ebs struct {
					Encrypted           bool
					DeleteOnTermination bool
					VolumeType          string
				}
			}
			if err := json.Unmarshal(resource.Properties["BlockDeviceMappings"], &disks); err != nil {
				t.Fatal(err)
			}
			if len(disks) != 1 || !disks[0].Ebs.Encrypted || !disks[0].Ebs.DeleteOnTermination || disks[0].Ebs.VolumeType != "gp3" {
				t.Error("expected one encrypted gp3 root disk deleted with host")
			}
			if !strings.Contains(string(resource.Properties["UserData"]), "OnBootSec=6h") {
				t.Error("missing independent boot lifetime cap")
			}
		}
		if resource.Type == "AWS::EC2::SecurityGroup" {
			var rules []struct {
				IpProtocol string
				FromPort   json.RawMessage
				CidrIp     json.RawMessage
			}
			if err := json.Unmarshal(resource.Properties["SecurityGroupIngress"], &rules); err != nil {
				t.Fatal(err)
			}
			for _, rule := range rules {
				expected := "OperatorCIDR"
				if strings.Contains(string(rule.FromPort), "UDPFrom") {
					expected = "GameCIDR"
				}
				if !strings.Contains(string(rule.CidrIp), expected) {
					t.Errorf("host port %s does not use %s", rule.FromPort, expected)
				}
			}
		}
	}
	if hosts != 1 {
		t.Fatalf("expected one shared host, got %d", hosts)
	}
}

func TestConsoleHostKeyRejectsAmbiguousIdentity(t *testing.T) {
	makeKey := func() string {
		public, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		key, err := ssh.NewPublicKey(public)
		if err != nil {
			t.Fatal(err)
		}
		return "INSTA_GAME_SSH_HOST_KEY=" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	}
	key := makeKey()
	for _, test := range []struct {
		name, output       string
		wantKey, wantError bool
	}{
		{"not ready", "booting\n", false, false},
		{"valid", "booting\n" + key + " ubuntu\r\n", true, false},
		{"duplicate", key + "\n" + key, true, false},
		{"conflicting", key + "\n" + makeKey(), false, true},
		{"malformed", "INSTA_GAME_SSH_HOST_KEY=ssh-ed25519 garbage", false, true},
		{"script text", "printf 'INSTA_GAME_SSH_HOST_KEY=%s'", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			found, err := consoleHostKey(test.output)
			if (err != nil) != test.wantError || (found != nil) != test.wantKey {
				t.Fatalf("key present=%v err=%v", found != nil, err)
			}
		})
	}
}

func TestDeploymentCanUpdateWhileHostSleeps(t *testing.T) {
	var template struct {
		Parameters map[string]struct{ Type string }
		Outputs    map[string]json.RawMessage
	}
	if err := json.Unmarshal([]byte(EnvironmentTemplate), &template); err != nil {
		t.Fatal(err)
	}
	if template.Parameters["UbuntuImage"].Type != "AWS::EC2::Image::Id" {
		t.Fatal("host image must stay pinned across deployment updates")
	}
	for name, value := range template.Outputs {
		if strings.Contains(string(value), "PublicIp") {
			t.Fatalf("output %s requires a running host", name)
		}
	}
}
