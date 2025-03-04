package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/georgemblack/blue-report/pkg/util"
)

// SaveThumbnail fetches an image at a given URL and stores it in S3.
// The identifier for the image is returned.
func (a AWS) SaveThumbnail(id string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return util.WrapErr("failed to fetch image", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch image, status code: %s", resp.Status)
	}
	defer resp.Body.Close()

	image, err := io.ReadAll(resp.Body)
	if err != nil {
		return util.WrapErr("failed to read image", err)
	}

	key := fmt.Sprintf("thumbnails/%s.jpg", id)
	_, err = a.s3.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:               aws.String(a.cfg.PublicBucketName),
		Key:                  aws.String(key),
		Body:                 bytes.NewReader(image),
		ServerSideEncryption: "AES256",
		ContentType:          aws.String("image/jpeg"),
		CacheControl:         aws.String("max-age=86400"), // 1 day
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	_, err = a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(a.cfg.PublicBucketName),
		Key:          aws.String(key),
		Body:         bytes.NewReader(image),
		ContentType:  aws.String("image/jpeg"),
		CacheControl: aws.String("max-age=86400"), // 1 day
	})
	if err != nil {
		return util.WrapErr("failed to put object", err)
	}

	return nil
}

// TODO: Move to R2 storage
func (a AWS) ThumbnailExists(id string) (bool, error) {
	_, err := a.s3.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(a.cfg.PublicBucketName),
		Key:    aws.String(fmt.Sprintf("thumbnails/%s.jpg", id)),
	})
	if err != nil {
		var notFound *s3Types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, util.WrapErr("failed to head object", err)
	}

	return true, nil
}
