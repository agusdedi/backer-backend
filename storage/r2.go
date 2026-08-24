package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	client       *s3.Client
	bucketName   string
	useLocalDisk bool
)

// Init sets up the storage backend based on STORAGE_DRIVER env var.
// "r2" (default) uploads to Cloudflare R2. "local" saves to disk,
// useful for local development without R2 credentials.
func Init() error {
	driver := os.Getenv("STORAGE_DRIVER")
	if driver == "local" {
		useLocalDisk = true
		return nil
	}

	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName = os.Getenv("R2_BUCKET_NAME")

	if accountID == "" || accessKey == "" || secretKey == "" || bucketName == "" {
		return fmt.Errorf("R2 credentials are not fully set")
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
		config.WithRegion("auto"),
	)
	if err != nil {
		return err
	}

	client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + accountID + ".r2.cloudflarestorage.com")
	})
	return nil
}

// UploadFile saves a file under the given key/path, using whichever
// backend is active (R2 or local disk).
func UploadFile(ctx context.Context, file io.Reader, key, contentType string) error {
	if useLocalDisk {
		return uploadLocal(file, key)
	}
	return uploadR2(ctx, file, key, contentType)
}

func uploadR2(ctx context.Context, file io.Reader, key, contentType string) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	return err
}

// uploadLocal writes the file to disk, mirroring what c.SaveUploadedFile
// used to do — key is used directly as the relative path.
func uploadLocal(file io.Reader, key string) error {
	if err := os.MkdirAll(filepath.Dir(key), 0755); err != nil {
		return fmt.Errorf("failed to create local upload dir: %w", err)
	}

	dst, err := os.Create(key)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}
	return nil
}
