package s3

import (
	"bytes"
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nimbolus/terraform-backend/terraform"
)

type S3Store struct {
	client *minio.Client
	bucket string
}

func NewS3Store(endpoint, bucket, accessKey, secretKey string, useSSL bool) (*S3Store, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize minio client: %v", err)
	}

	if exists, err := client.BucketExists(context.Background(), bucket); err != nil {
		return nil, fmt.Errorf("failed to check for bucket: %v", err)
	} else if !exists {
		return nil, fmt.Errorf("bucket does not exist")
	}

	return &S3Store{
		client: client,
		bucket: bucket,
	}, nil
}

func (s *S3Store) GetName() string {
	return "s3"
}

func (s *S3Store) SaveState(state *terraform.State) error {
	r := bytes.NewReader(state.Data)
	_, err := s.client.PutObject(context.Background(), s.bucket, getObjectName(state.ID), r, r.Size(), minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	return err
}

func (s *S3Store) GetState(id string) (*terraform.State, error) {
	state := &terraform.State{
		ID: id,
	}

	obj, err := s.client.GetObject(context.Background(), s.bucket, getObjectName(id), minio.GetObjectOptions{})
	if err != nil {
		return state, err
	}
	defer obj.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(obj); err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return state, nil
		}
		return state, err
	}

	state.Data = buf.Bytes()
	return state, nil
}

func getObjectName(id string) string {
	return fmt.Sprintf("%s.tfstate", id)
}
