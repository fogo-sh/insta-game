package cloud

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestAWSDestroyLobbyArtifacts(t *testing.T) {
	if os.Getenv("INSTA_GAME_AWS_CLEANUP_TEST") != "1" {
		t.Skip("set INSTA_GAME_AWS_CLEANUP_TEST=1 for isolated AWS cleanup fixtures")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sdk, err := LoadSDK(ctx, os.Getenv("AWS_REGION"))
	if err != nil {
		t.Fatal(err)
	}
	stacks := cloudformation.NewFromConfig(sdk)
	objects := s3.NewFromConfig(sdk)
	const template = `{"Resources":{"Code":{"Type":"AWS::S3::Bucket"},"Requests":{"Type":"AWS::DynamoDB::Table","Properties":{"BillingMode":"PAY_PER_REQUEST","AttributeDefinitions":[{"AttributeName":"id","AttributeType":"S"}],"KeySchema":[{"AttributeName":"id","KeyType":"HASH"}]}}},"Outputs":{"LobbyCodeBucket":{"Value":{"Ref":"Code"}}}}`
	for _, mode := range []string{"objects", "missing"} {
		t.Run(mode, func(t *testing.T) {
			name := "insta-game-lambda-cleanup-" + mode
			// Fixed fixture names must be absent; never adopt or replace an existing stack.
			if _, err := stacks.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(name)}); !MissingStack(err) {
				t.Fatalf("cleanup fixture name is occupied: %v", err)
			}
			result, err := stacks.CreateStack(ctx, &cloudformation.CreateStackInput{StackName: aws.String(name), TemplateBody: aws.String(template), Tags: []cf.Tag{{Key: aws.String("insta-game:kind"), Value: aws.String("shared-host")}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if err := Destroy(cleanup, sdk, aws.ToString(result.StackId)); err != nil {
					t.Error(err)
				}
			})
			if err := cloudformation.NewStackCreateCompleteWaiter(stacks).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: result.StackId}, 4*time.Minute); err != nil {
				t.Fatal(err)
			}
			stack, err := DescribeStack(ctx, stacks, name)
			if err != nil {
				t.Fatal(err)
			}
			bucket := Outputs(stack)["LobbyCodeBucket"]
			if mode == "objects" {
				if _, err := objects.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String("bootstrap.zip"), Body: bytes.NewReader([]byte("fixture"))}); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := objects.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
					t.Fatal(err)
				}
			}
			if err := Destroy(ctx, sdk, name); err != nil {
				t.Fatal(err)
			}
			if _, err := stacks.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: result.StackId}); MissingStack(err) {
				return
			} else if err != nil {
				t.Fatal(err)
			}
			// AWS may retain a deleted stack's record when addressed by ARN.
			resultStack, err := stacks.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: result.StackId})
			if err != nil {
				t.Fatal(err)
			}
			if len(resultStack.Stacks) != 1 || resultStack.Stacks[0].StackStatus != cf.StackStatusDeleteComplete {
				t.Fatal("fixture was not deleted")
			}
		})
	}
}
