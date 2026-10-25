package fleet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func testEC2(t *testing.T, handler http.HandlerFunc) *EC2 {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	sdk := aws.Config{Region: "ca-central-1", Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	})}
	return &EC2{Stack: "owned-stack", Client: ec2.NewFromConfig(sdk, func(o *ec2.Options) { o.BaseEndpoint = aws.String(server.URL) })}
}
func machineXML(h Host, stack string) string {
	return fmt.Sprintf(`<reservationSet><item><instancesSet><item><instanceId>i-owned</instanceId><clientToken>%s</clientToken><instanceState><name>running</name></instanceState><tagSet><item><key>insta-game:fleet</key><value>%s</value></item><item><key>insta-game:host</key><value>%s</value></item></tagSet></item></instancesSet></item></reservationSet>`, h.LaunchToken, stack, h.ID)
}
func TestEC2RecoversLaunchWithPinnedTokenAndRefusesForeignMachine(t *testing.T) {
	h := Host{ID: "host-a", LaunchToken: "immutable-token", LaunchTemplate: "lt-owned", LaunchVersion: "7"}
	launched := false
	launches := 0
	foreign := false
	mutations := 0
	provider := testEC2(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		action := r.Form.Get("Action")
		w.Header().Set("Content-Type", "text/xml")
		switch action {
		case "DescribeInstances":
			value := ""
			if launched {
				stack := "owned-stack"
				if foreign {
					stack = "foreign-stack"
				}
				value = machineXML(h, stack)
			}
			fmt.Fprint(w, "<DescribeInstancesResponse>"+value+"</DescribeInstancesResponse>")
		case "RunInstances":
			launches++
			if r.Form.Get("ClientToken") != h.LaunchToken || r.Form.Get("LaunchTemplate.Version") != "7" || r.Form.Get("MinCount") != "1" || r.Form.Get("MaxCount") != "1" {
				t.Errorf("launch spec was not pinned: %v", r.Form)
			}
			launched = true
			fmt.Fprint(w, "<RunInstancesResponse><instancesSet><item><instanceId>i-owned</instanceId></item></instancesSet></RunInstancesResponse>")
		default:
			mutations++
			io.WriteString(w, "<Response/>")
		}
	})
	for range 2 {
		id, err := provider.Wake(context.Background(), h)
		if err != nil || id != "i-owned" {
			t.Fatal(id, err)
		}
	}
	if launches != 1 {
		t.Fatal("retry created duplicate host")
	}
	foreign = true
	if err := provider.Stop(context.Background(), h); err == nil || !strings.Contains(err.Error(), "unmanaged") {
		t.Fatal("foreign machine was accepted", err)
	}
	if mutations != 0 {
		t.Fatal("foreign machine was mutated")
	}
}
func TestEC2CleanupDoesNotRecreateMissingRecordedHost(t *testing.T) {
	actions := []string{}
	provider := testEC2(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		action := r.Form.Get("Action")
		actions = append(actions, action)
		w.Header().Set("Content-Type", "text/xml")
		if action == "DescribeInstances" {
			w.WriteHeader(400)
			fmt.Fprint(w, `<Response><Errors><Error><Code>InvalidInstanceID.NotFound</Code><Message>gone</Message></Error></Errors></Response>`)
			return
		}
		if action != "DescribeVolumes" {
			t.Errorf("cleanup launched/mutated a missing recorded machine: %s", action)
		}
		fmt.Fprint(w, "<DescribeVolumesResponse><volumeSet/></DescribeVolumesResponse>")
	})
	if err := provider.Delete(context.Background(), Host{ID: "host-a", InstanceID: "i-recorded", LaunchToken: "token"}); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatal(actions)
	}
}

func TestEC2RequiresDurableLaunchIdentityBeforeRequest(t *testing.T) {
	provider := testEC2(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("sent an EC2 request without a durable launch token")
	})
	_, err := provider.Wake(context.Background(), Host{ID: "host-a", LaunchTemplate: "lt-owned", LaunchVersion: "1"})
	if err == nil {
		t.Fatal("launched without idempotency token")
	}
}
