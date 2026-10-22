package control

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/fogo-sh/insta-game/internal/cloud"
)

func instanceTemplate(environment cloud.Config, i Instance) (string, error) {
	if i.Game.AgentPort != 5001 {
		return "", fmt.Errorf("AWS management port must be 5001")
	}
	directory := i.Game.DataRoot
	if !path.IsAbs(directory) || directory == "/" || !strings.HasPrefix(i.Game.ConfigPath, path.Clean(directory)+"/") {
		return "", fmt.Errorf("AWS requires a dedicated absolute data directory")
	}
	ref := func(name string) map[string]string { return map[string]string{"Ref": name} }
	attr := func(name, key string) map[string]any { return map[string]any{"Fn::GetAtt": []string{name, key}} }
	sub := func(value string) map[string]string { return map[string]string{"Fn::Sub": value} }
	statement := func(actions []string, resource any) map[string]any {
		return map[string]any{"Effect": "Allow", "Action": actions, "Resource": resource}
	}
	mount := statement([]string{"elasticfilesystem:ClientMount", "elasticfilesystem:ClientWrite"}, sub("arn:${AWS::Partition}:elasticfilesystem:${AWS::Region}:${AWS::AccountId}:file-system/"+environment.FileSystem))
	mount["Condition"] = map[string]any{"StringEquals": map[string]any{"elasticfilesystem:AccessPointArn": attr("Data", "Arn")}}
	policies := []any{mount, statement([]string{"s3:GetObject"}, sub("arn:${AWS::Partition}:s3:::"+environment.AssetsBucket+"/*"))}
	env := map[string]any{
		"GAME_CMD": i.Game.GameCmd, "GAME_ARGS": i.Game.GameArgs, "GAME_QUIT_CMD": i.Game.GameQuitCmd,
		"GAME_QUIT_TIMEOUT": strconv.Itoa(i.Game.GameQuitTimeout), "GAME_PORT": strconv.Itoa(i.Game.GamePort),
		"PROTOCOL": i.Game.Protocol, "PORT": "5001", "CONFIG_PATH": i.Game.ConfigPath, "CONFIG_TEXT": i.Config, "DATA_ROOT": directory,
		"TOKEN": ref("AgentToken"), "RCON_PASSWORD": ref("RconPassword"), "INSTANCE_OWNER": owner(i), "AGENT_TLS": "true", "AWS_REGION": environment.Region,
	}
	if i.AWS.DataURL != "" {
		env["DATA_URL"] = i.AWS.DataURL
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	variables := []any{}
	for _, key := range keys {
		variables = append(variables, map[string]any{"Name": key, "Value": env[key]})
	}
	mappings := []any{map[string]any{"ContainerPort": 5001, "Protocol": "tcp"}}
	ingress := []any{}
	portKeys := make([]string, 0, len(i.Game.Ports))
	for key := range i.Game.Ports {
		portKeys = append(portKeys, key)
	}
	sort.Strings(portKeys)
	for _, key := range portKeys {
		parts := strings.Split(key, "/")
		number, err := strconv.Atoi(parts[0])
		if err != nil || len(parts) != 2 {
			return "", fmt.Errorf("invalid game port %s", key)
		}
		if number == 5001 && parts[1] == "tcp" {
			continue
		}
		mappings = append(mappings, map[string]any{"ContainerPort": number, "Protocol": parts[1]})
		ingress = append(ingress, map[string]any{"IpProtocol": parts[1], "FromPort": number, "ToPort": number, "CidrIp": "0.0.0.0/0"})
	}
	cpu, memory, architecture := i.Game.CPU, i.Game.Memory, i.Game.Architecture
	if cpu == 0 {
		cpu = 512
	}
	if memory == 0 {
		memory = 1024
	}
	if architecture == "" {
		architecture = "ARM64"
	}
	stopTimeout := min(max(i.Game.GameQuitTimeout+5, 20), 120)
	resources := map[string]any{
		"Data": map[string]any{"Type": "AWS::EFS::AccessPoint", "Properties": map[string]any{
			"FileSystemId": environment.FileSystem, "PosixUser": map[string]int{"Uid": 1000, "Gid": 1000},
			"RootDirectory": map[string]any{"Path": "/instances/" + owner(i), "CreationInfo": map[string]any{"OwnerUid": 1000, "OwnerGid": 1000, "Permissions": "0700"}},
		}},
		"GameSecurityGroup": map[string]any{"Type": "AWS::EC2::SecurityGroup", "Properties": map[string]any{"VpcId": environment.Network, "GroupDescription": "insta-game " + i.ID, "SecurityGroupIngress": ingress}},
		"TaskRole": map[string]any{"Type": "AWS::IAM::Role", "Properties": map[string]any{
			"AssumeRolePolicyDocument": map[string]any{"Version": "2012-10-17", "Statement": []any{map[string]any{"Effect": "Allow", "Principal": map[string]string{"Service": "ecs-tasks.amazonaws.com"}, "Action": "sts:AssumeRole"}}},
			"Policies":                 []any{map[string]any{"PolicyName": "instance-data", "PolicyDocument": map[string]any{"Version": "2012-10-17", "Statement": policies}}},
		}},
		"Definition": map[string]any{"Type": "AWS::ECS::TaskDefinition", "Properties": map[string]any{
			"RequiresCompatibilities": []string{"FARGATE"}, "NetworkMode": "awsvpc", "Cpu": strconv.Itoa(cpu), "Memory": strconv.Itoa(memory),
			"RuntimePlatform":  map[string]string{"CpuArchitecture": architecture, "OperatingSystemFamily": "LINUX"},
			"ExecutionRoleArn": environment.ExecutionRole, "TaskRoleArn": attr("TaskRole", "Arn"),
			"Volumes": []any{map[string]any{"Name": "data", "EFSVolumeConfiguration": map[string]any{"FilesystemId": environment.FileSystem, "TransitEncryption": "ENABLED", "AuthorizationConfig": map[string]any{"AccessPointId": ref("Data"), "IAM": "ENABLED"}}}},
			"ContainerDefinitions": []any{map[string]any{"Name": "game", "Image": i.Game.Image, "Essential": true, "EntryPoint": []string{"/usr/local/bin/insta-game"}, "Command": []string{"agent"}, "StopTimeout": stopTimeout,
				"Environment": variables, "PortMappings": mappings, "MountPoints": []any{map[string]any{"SourceVolume": "data", "ContainerPath": directory}},
				"LogConfiguration": map[string]any{"LogDriver": "awslogs", "Options": map[string]string{"awslogs-group": environment.LogGroup, "awslogs-region": environment.Region, "awslogs-stream-prefix": i.ID}},
			}},
		}},
	}
	body := map[string]any{"AWSTemplateFormatVersion": "2010-09-09", "Parameters": map[string]any{"AgentToken": map[string]any{"Type": "String", "NoEcho": true}, "RconPassword": map[string]any{"Type": "String", "NoEcho": true}}, "Resources": resources, "Outputs": map[string]any{
		"Definition": map[string]any{"Value": ref("Definition")}, "SecurityGroup": map[string]any{"Value": ref("GameSecurityGroup")},
	}}
	encoded, err := json.Marshal(body)
	return string(encoded), err
}
