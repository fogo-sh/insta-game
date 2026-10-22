package control

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fogo-sh/insta-game/internal/cloud"
)

func TestCloudFormationTemplates(t *testing.T) {
	if os.Getenv("INSTA_GAME_LINT_TEMPLATES") == "" {
		t.Skip("run make lint-templates for CloudFormation schema validation")
	}
	games, err := LoadCatalog("../../docker-containers")
	if err != nil {
		t.Fatal(err)
	}
	environment := cloud.Config{
		Region: "ca-central-1", Network: "vpc-0123456789abcdef0",
		FileSystem: "fs-0123456789abcdef0", AssetsBucket: "insta-game-lint-assets",
		ExecutionRole: "arn:aws:iam::123456789012:role/insta-game-lint",
		LogGroup:      "/insta-game/lint",
	}
	directory := t.TempDir()
	templates := map[string]string{"environment": cloud.EnvironmentTemplate}
	for _, game := range games {
		instance := Instance{ID: game.ID + "-lint", Game: game, AgentToken: "lint-owner", Config: game.DefaultConfig,
			AWS: AWSState{DataURL: "s3://insta-game-lint-assets/game.zip=/opt"}}
		template, err := instanceTemplate(environment, instance)
		if err != nil {
			t.Fatalf("%s: %v", game.ID, err)
		}
		templates[game.ID] = template
	}
	args := []string{"--regions", environment.Region, "--template"}
	for name, template := range templates {
		path := filepath.Join(directory, name+".json")
		if err := os.WriteFile(path, []byte(template), 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, path)
	}
	if output, err := exec.Command("cfn-lint", args...).CombinedOutput(); err != nil {
		t.Fatalf("CloudFormation template lint: %v\n%s", err, output)
	}
}
