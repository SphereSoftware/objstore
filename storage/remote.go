package storage

import (
	"fmt"
	"io"
	"mime"
	"path"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// RemoteStorage provides object access backend,
// it's usually an AWS S3 client pointed to a specific bucket.
type RemoteStorage interface {
	PutObject(key string, r io.ReadSeeker, meta map[string]string) (*Spec, error)
	GetObject(key string, version ...string) (*Spec, error)
	HeadObject(key string, version ...string) (*Spec, error)
	ListObjects(prefix string, startAfter ...string) ([]*Spec, error)
	CheckAccess(prefix string) error
	Bucket() string
}

type s3Storage struct {
	bucket string
	cli    *s3.S3
}

func NewS3Storage(region, bucket string) RemoteStorage {
	cli := s3.New(session.New(&aws.Config{
		Region: aws.String(region),
	}))
	return &s3Storage{
		bucket: bucket,
		cli:    cli,
	}
}

type Spec struct {
	Path      string
	Key       string
	Body      io.ReadCloser
	ETag      string
	Version   string
	UpdatedAt time.Time
	Meta      map[string]string
	Size      int64
}

func (s *s3Storage) Bucket() string {
	return s.bucket
}

func (s *s3Storage) GetObject(key string, version ...string) (*Spec, error) {
	obj, err := s.cli.GetObject(&s3.GetObjectInput{
		Key:       aws.String(key),
		Bucket:    aws.String(s.bucket),
		VersionId: awsStringMaybe(version),
	})
	if err != nil {
		return nil, err
	}
	spec := &Spec{
		Path:      fullPath(s.bucket, key),
		Key:       key,
		Body:      obj.Body,
		ETag:      *obj.ETag,
		Version:   *obj.VersionId,
		UpdatedAt: *obj.LastModified,
		Size:      *obj.ContentLength,
		Meta:      aws.StringValueMap(obj.Metadata),
	}
	return spec, nil
}

func (s *s3Storage) HeadObject(key string, version ...string) (*Spec, error) {
	obj, err := s.cli.HeadObject(&s3.HeadObjectInput{
		Key:       aws.String(key),
		Bucket:    aws.String(s.bucket),
		VersionId: awsStringMaybe(version),
	})
	if err != nil {
		return nil, err
	}
	spec := &Spec{
		Path:      fullPath(s.bucket, key),
		Key:       key,
		ETag:      *obj.ETag,
		Version:   *obj.VersionId,
		UpdatedAt: *obj.LastModified,
		Size:      *obj.ContentLength,
	}
	return spec, nil
}

func (s *s3Storage) ListObjects(prefix string, startAfter ...string) ([]*Spec, error) {
	var token *string
	var specs []*Spec
	for {
		list, err := s.cli.ListObjectsV2(&s3.ListObjectsV2Input{
			Bucket:     aws.String(s.bucket),
			Prefix:     aws.String(prefix),
			StartAfter: awsStringMaybe(startAfter),
			// pagination controls
			MaxKeys:           aws.Int64(100),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range list.Contents {
			specs = append(specs, &Spec{
				Path:      fullPath(s.bucket, *obj.Key),
				Key:       *obj.Key,
				ETag:      *obj.ETag,
				UpdatedAt: *obj.LastModified,
				Size:      *obj.Size,
			})
		}
		token = list.ContinuationToken
		if *list.IsTruncated == false {
			return specs, nil
		} else if token == nil {
			return specs, nil
		}
	}
	return specs, nil
}

func (s *s3Storage) CheckAccess(prefix string) error {
	body := []byte(time.Now().UTC().String())
	_, err := s.cli.PutObject(&s3.PutObjectInput{
		Body:        newReadSeeker(body),
		Bucket:      aws.String(s.bucket),
		ContentType: aws.String("text/plain"),
		Key:         aws.String(path.Join(prefix, "_objstore_touch")),
	})
	return err
}

func (s *s3Storage) PutObject(key string, r io.ReadSeeker, meta map[string]string) (*Spec, error) {
	var ctype string
	if len(meta["name"]) > 0 {
		ctype = mime.TypeByExtension(filepath.Ext(meta["name"]))
	}
	obj, err := s.cli.PutObject(&s3.PutObjectInput{
		Body:        r,
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(ctype),
		Metadata:    aws.StringMap(meta),
	})
	if err != nil {
		return nil, err
	}
	spec := &Spec{
		Path:    fullPath(s.bucket, key),
		Key:     key,
		ETag:    *obj.ETag,
		Version: *obj.VersionId,
		Meta:    meta,
	}
	return spec, err
}

func fullPath(bucket, key string) string {
	return fmt.Sprintf("s3://%s/%s", bucket, key)
}

func awsStringMaybe(v []string) *string {
	if len(v) > 0 {
		return aws.String(v[0])
	}
	return nil
}
