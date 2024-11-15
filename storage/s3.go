package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3 Object
type S3 struct {
	Bucket string
	Client *minio.Client
	Ctx    context.Context
}

// Create a new S3 Object
// ctx - Pass your telemetry context here
func New(ctx context.Context) (*S3, error) {
	// Check to see if SSL is enabled with S3
	ssl, err := strconv.ParseBool(os.Getenv("GODISTCACHE_S3_SSL"))
	if err != nil {
		return nil, err
	}
	// Create new S3 client
	client, err := minio.New(os.Getenv("GODISTCACHE_S3_ENDPOINT"), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("GODISTCACHE_S3_ACCESS_KEY"), os.Getenv("GODISTCACHE_S3_SECRET_KEY"), ""),
		Secure: ssl,
	})
	if err != nil {
		return nil, err
	}
	return &S3{
		Bucket: os.Getenv("GODISTCACHE_S3_BUCKET"),
		Client: client,
		Ctx:    ctx,
	}, nil
}

// This will download a file from an S3 compatible storage server
// key -> The objects key in S3 -> Do not include the .godistcache extension
// This was tested with SeaweedFS S3
func (s3 *S3) S3Download(key string) (string, error) {
	// Get the Object from S3
	object, err := s3.Client.GetObject(s3.Ctx, s3.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer object.Close()
	// Get current working directory
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Get current time and path
	t := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	path := pwd + "/" + t
	cacheFile, err := os.Create(path + ".godistcache")
	if err != nil {
		return "", err
	}
	defer cacheFile.Close()
	// Copy to file
	if _, err = io.Copy(cacheFile, object); err != nil {
		return "", err
	}
	return path, err
}

// This will upload the file to S3 to the master file as we as the current days backup under the current instance
// filePathName -> The path with the filename - DO NOT add the extension .godistcache
// key -> The objects key in S3 -> Do not include the .godistcache extension
func (s3 *S3) S3Upload(filePathName, key string) error {
	if s3.Bucket == "" {
		return errors.New("Bucket is nil")
	}
	id := os.Getenv("GODISTCACHE_INSTANCE_ID")
	file, err := os.Open(filePathName + ".godistcache")
	if err != nil {
		return err
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		return err
	}
	t := time.Now().Format("01-02-2006")
	// Create an entry for today
	_, err = s3.Client.PutObject(s3.Ctx, s3.Bucket, key+"_"+id+"_"+t+".godistcache", file, fileStat.Size(), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return err
	}
	// Copy to "Master"
	src := minio.CopySrcOptions{
		Bucket: os.Getenv("GODISTCACHE_S3_BUCKET"),
		Object: key + "_" + id + "_" + t + ".godistcache",
	}
	dst := minio.CopyDestOptions{
		Bucket: os.Getenv("GODISTCACHE_S3_BUCKET"),
		Object: key + ".godistcache",
	}
	_, err = s3.Client.CopyObject(s3.Ctx, dst, src)
	if err != nil {
		return err
	}
	return nil
}
