package app

import (
	"time"

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
	PublishSite(site []byte) error
	PublishArchive(site []byte) error
	PublishSnapshot(snapshot []byte) error
	ReadEvents(key string) ([]storage.EventRecord, error)
	FlushEvents(start time.Time, events []storage.EventRecord) error
	ListEventChunks(start, end time.Time) ([]string, error)
	SaveThumbnail(id string, url string) error
	ThumbnailExists(id string) (bool, error)
}

type Secrets interface {
	GetDeployHook() (string, error)
}
