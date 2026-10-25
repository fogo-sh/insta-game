package cloud

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/fogo-sh/insta-game/internal/control"
	"github.com/fogo-sh/insta-game/internal/fleet"
)

func TestFleetUpdateUsesDeployedHardwareAndLimits(t *testing.T) {
	stack := &cf.Stack{Parameters: []cf.Parameter{{ParameterKey: aws.String("InstanceType"), ParameterValue: aws.String("t4g.medium")}, {ParameterKey: aws.String("DiskGiB"), ParameterValue: aws.String("32")}, {ParameterKey: aws.String("MaxHosts"), ParameterValue: aws.String("2")}}}
	defaults, err := fleetDefaults(stack)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Capacity.CPU != 350 || defaults.Capacity.MemoryMiB != 3200 || defaults.Capacity.DiskMiB != 20*1024 || defaults.MaxHosts != 2 {
		t.Fatal("capacity was derived from defaults instead of deployed hardware", defaults)
	}
	params, err := updateParameters(FleetTemplate, append(stack.Parameters, cf.Parameter{ParameterKey: aws.String("RemovedParameter")}), map[string]string{"Release": "immutable-release"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range params {
		if aws.ToString(p.ParameterKey) == "RemovedParameter" {
			t.Fatal("replayed removed template parameter")
		}
		if aws.ToString(p.ParameterKey) == "MaxHosts" && !aws.ToBool(p.UsePreviousValue) {
			t.Fatal("lost reviewed host limit")
		}
	}
	if _, err = updateParameters(FleetTemplate, stack.Parameters, map[string]string{"MisspelledLimit": "5"}); err == nil {
		t.Fatal("ignored unknown parameter")
	}
}
func TestDefaultReservationsIncludeMeasuredHeadroom(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "capacity-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Profiles []struct {
			Game string  `json:"game"`
			CPU  float64 `json:"observed_max_run_p95_millicores"`
			RAM  float64 `json:"observed_max_run_p95_memory_mib"`
		}
	}
	if err = json.Unmarshal(data, &evidence); err != nil {
		t.Fatal(err)
	}
	for _, entry := range evidence.Profiles {
		reserved, ok := defaultRequirements[entry.Game]
		if !ok || float64(reserved.CPU) < 1.25*entry.CPU || float64(reserved.MemoryMiB) < 1.25*entry.RAM {
			t.Errorf("%s reservation is below measured p95 plus25%%", entry.Game)
		}
	}
}
func TestFleetTemplateHasNoStandingHostAndOwnsRetainedDisks(t *testing.T) {
	var template struct {
		Resources map[string]struct {
			Type       string
			Properties json.RawMessage
		}
	}
	if err := json.Unmarshal([]byte(FleetTemplate), &template); err != nil {
		t.Fatal(err)
	}
	for name, r := range template.Resources {
		switch r.Type {
		case "AWS::EC2::Instance", "AWS::EC2::NatGateway", "AWS::EC2::EIP", "AWS::ElasticLoadBalancingV2::LoadBalancer":
			t.Errorf("standing paid resource %s", name)
		}
	}
	var launch struct {
		LaunchTemplateData struct {
			CreditSpecification struct{ CpuCredits string }
			BlockDeviceMappings []struct {
				Ebs struct{ Encrypted, DeleteOnTermination bool }
			}
		}
	}
	if err := json.Unmarshal(template.Resources["LaunchTemplate"].Properties, &launch); err != nil {
		t.Fatal(err)
	}
	if launch.LaunchTemplateData.CreditSpecification.CpuCredits != "standard" {
		t.Fatal("surplus CPU billing enabled")
	}
	for _, d := range launch.LaunchTemplateData.BlockDeviceMappings {
		if !d.Ebs.Encrypted || !d.Ebs.DeleteOnTermination {
			t.Fatal("saved disks lack encryption or cleanup")
		}
	}
}

func TestWorkloadCompatibilityIgnoresPageReleaseAndBudgetChanges(t *testing.T) {
	original := FleetRelease{Catalog: []control.Game{{ID: "quake", DisplayName: "Quake", Image: "image@sha256:old", GameArgs: "-dedicated", DefaultConfig: "map dm4"}}, Assets: map[string]string{"quake": "assets/old.zip"}, Config: fleet.Config{IdleMinutes: 15, MaxMinutes: 360}}
	fingerprint := workloadFingerprint(original)
	changed := original
	changed.Config.Release = "new-binary"
	changed.Config.MaxHosts = 8
	changed.Config.Deadline = 12345
	if workloadFingerprint(changed) != fingerprint {
		t.Fatal("page/budget update made compatible hosts unusable")
	}
	changed = original
	changed.Assets = map[string]string{"quake": "assets/new.zip"}
	if workloadFingerprint(changed) == fingerprint {
		t.Fatal("asset change reused old host")
	}
	changed = original
	changed.Config.IdleMinutes = 30
	if workloadFingerprint(changed) == fingerprint {
		t.Fatal("lifecycle change reused old host defaults")
	}
	changed = original
	changed.Catalog = append([]control.Game(nil), original.Catalog...)
	changed.Catalog[0].Image = "image@sha256:new"
	if workloadFingerprint(changed) == fingerprint {
		t.Fatal("engine update reused old image")
	}
}

func TestFleetCodeUpdatePreservesDeployedPolicyAndCopiesInputs(t *testing.T) {
	stack := &cf.Stack{Parameters: []cf.Parameter{{ParameterKey: aws.String("InstanceType"), ParameterValue: aws.String("t4g.small")}, {ParameterKey: aws.String("DiskGiB"), ParameterValue: aws.String("32")}, {ParameterKey: aws.String("MaxHosts"), ParameterValue: aws.String("4")}}}
	config := DefaultFleetConfig(2, 32)
	config.IdleMinutes, config.MaxMinutes, config.Deadline = 3, 40, 1234567890
	config.LaunchTemplate, config.LaunchVersion = "lt-pinned", "2"
	config.Games = []fleet.Game{{ID: "fteqw", Name: "Quake", Resources: fleet.Resources{CPU: 90, MemoryMiB: 128, DiskMiB: 300}}}
	previous := &FleetRelease{Config: config, Catalog: []control.Game{{ID: "fteqw", DisplayName: "Quake", Image: "pinned@sha256:unchanged", Ports: map[string]json.RawMessage{"26000/udp": json.RawMessage(`{}`)}}}, Assets: map[string]string{"fteqw": "assets/retained.zip"}}
	defaults, err := fleetUpdateDefaults(stack, previous)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.MaxHosts != 2 || defaults.IdleMinutes != 3 || defaults.MaxMinutes != 40 || defaults.Deadline != 1234567890 || defaults.Games[0].Resources.CPU != 90 {
		t.Fatal("code update reset explicit runtime, expiry, host limit, or resources", defaults)
	}
	prepared, err := prepareFleetRelease(FleetReleaseOptions{Config: defaults}, previous)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Catalog[0].Image != previous.Catalog[0].Image || prepared.Assets["fteqw"] != "assets/retained.zip" {
		t.Fatal("code-only release did not reuse durable catalog/assets")
	}
	prepared.Assets["fteqw"] = "changed"
	prepared.Catalog[0].Ports["26000/udp"][0] = '['
	prepared.Config.Games[0].Name = "Changed"
	if previous.Assets["fteqw"] != "assets/retained.zip" || string(previous.Catalog[0].Ports["26000/udp"]) != `{}` || previous.Config.Games[0].Name != "Quake" {
		t.Fatal("preparing an update mutated deployed release inputs")
	}
	stack.Parameters[2].ParameterValue = aws.String("1")
	defaults, err = fleetUpdateDefaults(stack, previous)
	if err != nil || defaults.MaxHosts != 1 {
		t.Fatal("reviewed stack limit was not clamped", defaults, err)
	}
	previous.Config.Capacity.MemoryMiB = 3200
	if _, err := fleetUpdateDefaults(stack, previous); err == nil {
		t.Fatal("stale capacity exceeded actual hardware")
	}
}

func TestFleetAssetRequirementsAllowDurableReuseAndPartialOverlay(t *testing.T) {
	config := DefaultFleetConfig(2, 32)
	previous := &FleetRelease{Catalog: []control.Game{{ID: "fteqw", DisplayName: "Quake"}, {ID: "ut99", DisplayName: "Unreal"}}, Assets: map[string]string{"fteqw": "assets/retained.zip"}}
	if _, err := prepareFleetRelease(FleetReleaseOptions{Config: config}, nil); err == nil {
		t.Fatal("initial release accepted missing catalog")
	}
	if _, err := prepareFleetRelease(FleetReleaseOptions{Config: config}, previous); err == nil {
		t.Fatal("new commercial game accepted without durable assets")
	}
	assets := t.TempDir()
	if err := os.WriteFile(filepath.Join(assets, "ut99.zip"), []byte("archive checked by extractor"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareFleetRelease(FleetReleaseOptions{Config: config, Assets: assets}, previous)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Assets["fteqw"] != "assets/retained.zip" {
		t.Fatal("partial overlay discarded existing durable asset")
	}
	previous.Assets["ut99"] = "assets/unreal.zip"
	if _, err := prepareFleetRelease(FleetReleaseOptions{Config: config}, previous); err != nil {
		t.Fatal("durable commercial data required a local archive again", err)
	}
}

func TestWorkloadCompatibilityIgnoresCatalogDisplayOrder(t *testing.T) {
	original := FleetRelease{Catalog: []control.Game{{ID: "quake", DisplayName: "A", Image: "image-one"}, {ID: "unreal", DisplayName: "Z", Image: "image-two"}}}
	changed := FleetRelease{Catalog: []control.Game{{ID: "unreal", DisplayName: "A", Image: "image-two"}, {ID: "quake", DisplayName: "Z", Image: "image-one"}}}
	if workloadFingerprint(original) != workloadFingerprint(changed) {
		t.Fatal("display-name ordering made the same workload incompatible")
	}
}
