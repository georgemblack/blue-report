package app

import "github.com/georgemblack/blue-report/pkg/cache"

type Cache interface {
	SaveURL(hash string, url cache.CacheURLRecord) error
	ReadURL(hash string) (cache.CacheURLRecord, error)
	SavePost(hash string, post cache.CachePostRecord) error
	ReadPost(hash string) (cache.CachePostRecord, error)
	Close()
}
