package agent

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func fetchS3(raw string) (string, error) {
	location, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if location.Host == "" || location.Path == "" || location.RawQuery != "" {
		return "", fmt.Errorf("expected s3://bucket/object")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	sdk, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	result, err := s3.NewFromConfig(sdk).GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(location.Host), Key: aws.String(strings.TrimPrefix(location.Path, "/"))})
	if err != nil {
		return "", err
	}
	defer result.Body.Close()
	return streamToFile(result.Body, aws.ToInt64(result.ContentLength), raw)
}
