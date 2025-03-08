package app

import (
	"time"

	"github.com/georgemblack/blue-report/pkg/bluesky"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/queue"
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
	SaveURLTranslation(translation storage.URLTranslation) error
	GetURLTranslations() (map[string]string, error)
	AddFeedEntry(entry storage.FeedEntry) error
	GetFeedEntries() ([]storage.FeedEntry, error)
	PublishFeeds(atom, json string) error
	RecentFeedEntry() bool
}

type Queue interface {
	Send(message queue.Message) error
	Receive() ([]queue.Message, error)
}

type Bluesky interface {
	GetPost(atURI string) (bluesky.Post, error)
}
