package s3

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	yaml "gopkg.in/yaml.v2"

	"github.com/zhouhaibing089/k8s-pod-logs/pkg/storage"
)

func New(config string) (storage.Interface, error) {
	data, err := ioutil.ReadFile(config)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %s", config, err)
	}
	var cfg Config
	err = yaml.NewDecoder(strings.NewReader(string(data))).Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %s", err)
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to new minio client: %s", err)
	}

	return &s3{client: client, bucket: cfg.Bucket}, nil
}

// Config describes the information needed in order to access NuObject
type Config struct {
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type s3 struct {
	client *minio.Client
	bucket string
}

func (s *s3) List() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objects := []string{}
	for object := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{}) {
		objects = append(objects, object.Key)
	}

	return objects, nil
}

func (s *s3) Put(key string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to put object: %s", err)
	}

	return nil
}

func (s *s3) Get(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %s", err)
	}

	data, err := ioutil.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %s", err)
	}

	return data, nil
}

func (s *s3) Has(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		merr, ok := err.(minio.ErrorResponse)
		if ok && merr.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat object: %s", err)
	}

	return true, nil
}
