package s3

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Hell077/HireRadar/apps/backend/internal/config"
	"github.com/Hell077/HireRadar/apps/backend/internal/resume/application"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage struct {
	client    *awss3.Client
	presigner *awss3.PresignClient
	bucket    string
}

func New(ctx context.Context, cfg config.Config) (*Storage, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")))
	if err != nil {
		return nil, fmt.Errorf("load S3 configuration: %w", err)
	}
	newClient := func(endpoint string) *awss3.Client {
		return awss3.NewFromConfig(awsCfg, func(options *awss3.Options) { options.BaseEndpoint = aws.String(endpoint); options.UsePathStyle = true })
	}
	client := newClient(cfg.S3Endpoint)
	return &Storage{client: client, presigner: awss3.NewPresignClient(newClient(cfg.S3PublicEndpoint)), bucket: cfg.S3Bucket}, nil
}

func (s *Storage) PresignUpload(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	request, err := s.presigner.PresignPutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType)}, func(options *awss3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", fmt.Errorf("presign S3 upload: %w", err)
	}
	return request.URL, nil
}

func (s *Storage) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	request, err := s.presigner.PresignGetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, func(options *awss3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", fmt.Errorf("presign S3 download: %w", err)
	}
	return request.URL, nil
}

func (s *Storage) Stat(ctx context.Context, key string) (application.ObjectInfo, error) {
	result, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return application.ObjectInfo{}, fmt.Errorf("inspect S3 object: %w", err)
	}
	info := application.ObjectInfo{}
	if result.ContentLength != nil {
		info.Size = *result.ContentLength
	}
	if result.ContentType != nil {
		info.ContentType = *result.ContentType
	}
	return info, nil
}

func (s *Storage) PDFSignature(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Range: aws.String("bytes=0-7")})
	if err != nil {
		return nil, fmt.Errorf("read S3 object signature: %w", err)
	}
	defer result.Body.Close()
	bytes, err := io.ReadAll(io.LimitReader(result.Body, 8))
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("delete S3 object: %w", err)
	}
	return nil
}
