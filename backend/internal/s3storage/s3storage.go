// Package s3storage implements a storage backend (adapted to
// storage.Backend by internal/storage's dispatch — see that package's
// adapters.go) against any S3-compatible object store: AWS S3 itself, or a
// self-hosted/third-party service that speaks the S3 API (MinIO, Backblaze
// B2, Wasabi, Cloudflare R2, ...) via a configurable endpoint and optional
// path-style addressing.
//
// Security note: Target.SecretAccessKey is decrypted plaintext held only in
// memory. Like azureblob's SAS token, it must never be logged and must never
// be interpolated into an error via %v/%+v on the Target struct.
package s3storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BlobInfo describes a single stored object, mirroring azureblob.BlobInfo
// and storage.BlobInfo in shape. This package defines its own copy (rather
// than importing internal/storage) so that internal/storage can import this
// package to build its dispatch.NewBackend without creating an import
// cycle; internal/storage/adapters.go converts between the two shapes.
type BlobInfo struct {
	Name         string
	LastModified time.Time
	SizeBytes    int64
}

// Target identifies where to upload/list/delete objects and how to reach
// them. SecretAccessKey is the decrypted secret; it is only ever held in
// memory for the duration of a single request.
type Target struct {
	// Endpoint, when non-empty, overrides the default AWS endpoint
	// resolution — used to point at MinIO, Backblaze, Wasabi, R2, or any
	// other S3-compatible service. Empty means "use AWS's normal endpoint
	// for Region".
	Endpoint string
	Region   string
	Bucket   string

	AccessKeyID     string
	SecretAccessKey string

	// UsePathStyle selects path-style addressing
	// (http://host/bucket/key) instead of virtual-hosted-style
	// (http://bucket.host/key). Required by MinIO and most self-hosted
	// S3-compatible servers; must stay false for real AWS S3.
	UsePathStyle bool
}

type Backend struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
}

// NewBackend builds an S3 client for t and returns a storage.Backend bound
// to t.Bucket.
func NewBackend(t Target) (*Backend, error) {
	if t.Bucket == "" {
		return nil, fmt.Errorf("s3storage: Bucket must not be empty")
	}

	region := t.Region
	if region == "" {
		// The SDK requires a non-empty region even against non-AWS
		// endpoints that ignore it; "us-east-1" is the conventional
		// placeholder used by every S3-compatible server's own docs/examples.
		region = "us-east-1"
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(t.AccessKeyID, t.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("s3storage: load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if t.Endpoint != "" {
			o.BaseEndpoint = aws.String(t.Endpoint)
		}
		o.UsePathStyle = t.UsePathStyle
	})

	return &Backend{
		client:   client,
		uploader: manager.NewUploader(client),
		bucket:   t.Bucket,
	}, nil
}

// UploadStream uploads r as an object named blobName, streaming via a
// multi-part upload manager so arbitrary-size backups are never buffered in
// memory in full. Returns the number of bytes uploaded as reported by a
// follow-up HeadObject call (the upload response itself does not reliably
// carry content length for multi-part uploads).
func (b *Backend) UploadStream(ctx context.Context, blobName string, r io.Reader) (int64, error) {
	_, err := b.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(blobName),
		Body:   r,
	})
	if err != nil {
		return 0, fmt.Errorf("s3storage: upload: %w", err)
	}

	head, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(blobName),
	})
	if err != nil {
		return 0, fmt.Errorf("s3storage: head object after upload: %w", err)
	}
	if head.ContentLength == nil {
		return 0, nil
	}
	return *head.ContentLength, nil
}

// ListBlobs lists all objects under prefix, paginating through the full
// result set.
func (b *Backend) ListBlobs(ctx context.Context, prefix string) ([]BlobInfo, error) {
	var out []BlobInfo

	paginator := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3storage: list objects page: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			var size int64
			if obj.Size != nil {
				size = *obj.Size
			}
			var mtime time.Time
			if obj.LastModified != nil {
				mtime = *obj.LastModified
			}
			out = append(out, BlobInfo{
				Name:         *obj.Key,
				LastModified: mtime,
				SizeBytes:    size,
			})
		}
	}
	return out, nil
}

// CheckAccess validates that the bucket/credentials are usable without
// uploading or deleting anything: a lightweight ListObjectsV2 call capped at
// one result.
func (b *Backend) CheckAccess(ctx context.Context) error {
	one := int32(1)
	_, err := b.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.bucket),
		MaxKeys: aws.Int32(one),
	})
	if err != nil {
		return fmt.Errorf("s3storage: check access: %w", err)
	}
	return nil
}

// DeleteBlob permanently deletes an object. Irreversible — see
// the package doc comment.
func (b *Backend) DeleteBlob(ctx context.Context, blobName string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(blobName),
	})
	if err != nil {
		return fmt.Errorf("s3storage: delete object: %w", err)
	}
	return nil
}
