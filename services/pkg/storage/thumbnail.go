package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/georgemblack/blue-report/pkg/util"
)

var OpenGraphImageTypes = []string{"image/jpeg", "image/png", "image/gif"}
var OpenGraphImageExtensions = []string{"jpg", "png", "gif"}

const ThumbnailHostPrefix = "https://data.theblue.report/thumbnails"

// SaveThumbnail fetches an image at a given URL and stores it in S3.
// The identifier for the image is returned.
func (a AWS) SaveThumbnail(id string, imageURL string) (string, error) {
	// Fetch the image
	resp, err := http.Get(imageURL)
	if err != nil {
		return "", util.WrapErr("failed to fetch image", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch image, status code: %s", resp.Status)
	}
	defer resp.Body.Close()

	image, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", util.WrapErr("failed to read image", err)
	}

	// Determine the image's mime-type using the 'Content-Type' response header.
	// OpenGraph images can be PNGs, JPEGs, or GIFs.
	mimeType := resp.Header.Get("Content-Type")
	if !util.ContainsStr(OpenGraphImageTypes, mimeType) {
		// Attempt to detect the mime-type automatically
		mimeType = http.DetectContentType(image)

		// If it's still not a supported type, default to 'image/jpeg'
		if !util.ContainsStr(OpenGraphImageTypes, mimeType) {
			slog.Warn("unable to determine image mime-type", "url", imageURL)
			mimeType = "image/jpeg"
		}
	}

	key := fmt.Sprintf("thumbnails/%s.%s", id, extension(mimeType))
	_, err = a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(a.cfg.PublicBucketName),
		Key:          aws.String(key),
		Body:         bytes.NewReader(image),
		ContentType:  aws.String(mimeType),
		CacheControl: aws.String("max-age=28800"), // 8 hours
	})
	if err != nil {
		return "", util.WrapErr("failed to put object", err)
	}

	return fmt.Sprintf("%s/%s.%s", ThumbnailHostPrefix, id, extension(mimeType)), nil
}

// GetThumbnailURL checks whether a thumbnail exists in R2 storage. If it does, return the URL.
// Check for each of three possible image types: PNG, JPEG, and GIF.
func (a AWS) GetThumbnailURL(id string) (string, error) {
	for _, ext := range OpenGraphImageExtensions {
		_, err := a.r2.HeadObject(context.Background(), &s3.HeadObjectInput{
			Bucket: aws.String(a.cfg.PublicBucketName),
			Key:    aws.String(fmt.Sprintf("thumbnails/%s.%s", id, ext)),
		})
		if err != nil {
			var notFound *s3Types.NotFound
			if errors.As(err, &notFound) {
				continue
			}
			return "", util.WrapErr("failed to head object", err)
		}

		return fmt.Sprintf("%s/%s.%s", ThumbnailHostPrefix, id, ext), nil
	}

	return "", nil
}

func extension(mimeType string) string {
	switch mimeType {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	default:
		return "jpg"
	}
}
