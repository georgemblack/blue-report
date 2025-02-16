package app

import (
	"time"

	"github.com/georgemblack/blue-report/pkg/bluesky"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/storage"
)

type Cache interface {
	SaveURL(hash string, url cache.URLRecord) error
	ReadURL(hash string) (cache.URLRecord, error)
	RefreshURL(hash string) error
	SavePost(hash string, post cache.PostRecord) error
	ReadPost(hash string) (cache.PostRecord, error)
	RefreshPost(hash string) error
	Close()
}

type Storage interface {
	PublishLinkSnapshot(snapshot []byte) error
	PublishSiteSnapshot(snapshot []byte) error
	ReadEvents(key string, eventBufferSize int) ([]storage.EventRecord, error)
	FlushEvents(start time.Time, events []storage.EventRecord) error
	ListEventChunks(start, end time.Time) ([]string, error)
	SaveThumbnail(id string, url string) error
	ThumbnailExists(id string) (bool, error)
	GetURLMetadata(url string) (storage.URLMetadata, error)
	SaveURLMetadata(metadata storage.URLMetadata) error
}

type Secrets interface {
	GetDeployHook() (string, error)
}

type Bluesky interface {
	GetPost(atURI string) (bluesky.Post, error)
}
