package app

import (
	"github.com/georgemblack/blue-report/pkg/bluesky"
	"github.com/georgemblack/blue-report/pkg/cache"
	"github.com/georgemblack/blue-report/pkg/config"
	"github.com/georgemblack/blue-report/pkg/storage"
)

// App creates a new instance of the application, initializing the cache, storage, and Bluesky API client.
type App struct {
	Config  config.Config
	Cache   Cache
	Storage Storage
	Bluesky Bluesky
}

func NewApp() (App, error) {
	config, err := config.New()
	if err != nil {
		return App{}, err
	}

	cache, err := cache.New(config)
	if err != nil {
		return App{}, err
	}

	storage, err := storage.New(config)
	if err != nil {
		return App{}, err
	}

	bluesky := bluesky.New(config)

	return App{
		Config:  config,
		Cache:   cache,
		Storage: storage,
		Bluesky: bluesky,
	}, nil
}

func (a App) Close() {
	a.Cache.Close()
}
