package mediastore

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store is an S3/MinIO-backed Store. Objects are stored in bucket and
// accessed via publicBase (e.g. "http://localhost:9000/ruammit-media").
type S3Store struct {
	client     *s3.Client
	bucket     string
	publicBase string
}

// NewS3 builds an S3Store. Pass a non-empty endpoint to target MinIO or any
// S3-compatible service; leave it empty to use AWS S3 directly.
func NewS3(endpoint, region, bucket, accessKey, secretKey string) (*S3Store, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	opts := []func(*s3.Options){}
	if endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // required for MinIO
		})
	}

	var publicBase string
	if endpoint != "" {
		publicBase = strings.TrimRight(endpoint, "/") + "/" + bucket
	} else {
		publicBase = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)
	}

	return &S3Store{
		client:     s3.NewFromConfig(cfg, opts...),
		bucket:     bucket,
		publicBase: publicBase,
	}, nil
}

// Save uploads r under key and returns its public URL.
func (s *S3Store) Save(key string, r io.Reader) (string, error) {
	clean := cleanKey(key)
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(clean),
		Body:        r,
		ContentType: aws.String(contentType(clean)),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put %s: %w", clean, err)
	}
	return s.publicBase + "/" + clean, nil
}

// RemoveAll deletes every object whose key starts with prefix.
func (s *S3Store) RemoveAll(prefix string) error {
	clean := cleanKey(prefix)
	if clean == "" {
		return nil
	}

	var toDelete []types.ObjectIdentifier
	pager := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(clean),
	})
	for pager.HasMorePages() {
		page, err := pager.NextPage(context.Background())
		if err != nil {
			return fmt.Errorf("s3 list %s: %w", clean, err)
		}
		for _, obj := range page.Contents {
			toDelete = append(toDelete, types.ObjectIdentifier{Key: obj.Key})
		}
	}
	if len(toDelete) == 0 {
		return nil
	}

	_, err := s.client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &types.Delete{Objects: toDelete},
	})
	if err != nil {
		return fmt.Errorf("s3 delete %s: %w", clean, err)
	}
	return nil
}

func contentType(key string) string {
	switch strings.ToLower(filepath.Ext(key)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	default:
		return "application/octet-stream"
	}
}
