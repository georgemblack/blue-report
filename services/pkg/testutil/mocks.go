// Code generated by MockGen. DO NOT EDIT.
// Source: pkg/app/interface.go
//
// Generated by this command:
//
//	mockgen -source=pkg/app/interface.go -destination=pkg/testutil/mocks.go -package=testutil
//

// Package testutil is a generated GoMock package.
package testutil

import (
	reflect "reflect"
	time "time"

	bluesky "github.com/georgemblack/blue-report/pkg/bluesky"
	cache "github.com/georgemblack/blue-report/pkg/cache"
	queue "github.com/georgemblack/blue-report/pkg/queue"
	storage "github.com/georgemblack/blue-report/pkg/storage"
	gomock "go.uber.org/mock/gomock"
)

// MockCache is a mock of Cache interface.
type MockCache struct {
	ctrl     *gomock.Controller
	recorder *MockCacheMockRecorder
	isgomock struct{}
}

// MockCacheMockRecorder is the mock recorder for MockCache.
type MockCacheMockRecorder struct {
	mock *MockCache
}

// NewMockCache creates a new mock instance.
func NewMockCache(ctrl *gomock.Controller) *MockCache {
	mock := &MockCache{ctrl: ctrl}
	mock.recorder = &MockCacheMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCache) EXPECT() *MockCacheMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockCache) Close() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Close")
}

// Close indicates an expected call of Close.
func (mr *MockCacheMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockCache)(nil).Close))
}

// ReadPost mocks base method.
func (m *MockCache) ReadPost(hash string) (cache.PostRecord, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadPost", hash)
	ret0, _ := ret[0].(cache.PostRecord)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadPost indicates an expected call of ReadPost.
func (mr *MockCacheMockRecorder) ReadPost(hash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadPost", reflect.TypeOf((*MockCache)(nil).ReadPost), hash)
}

// ReadURL mocks base method.
func (m *MockCache) ReadURL(hash string) (cache.URLRecord, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadURL", hash)
	ret0, _ := ret[0].(cache.URLRecord)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadURL indicates an expected call of ReadURL.
func (mr *MockCacheMockRecorder) ReadURL(hash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadURL", reflect.TypeOf((*MockCache)(nil).ReadURL), hash)
}

// RefreshPost mocks base method.
func (m *MockCache) RefreshPost(hash string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshPost", hash)
	ret0, _ := ret[0].(error)
	return ret0
}

// RefreshPost indicates an expected call of RefreshPost.
func (mr *MockCacheMockRecorder) RefreshPost(hash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshPost", reflect.TypeOf((*MockCache)(nil).RefreshPost), hash)
}

// RefreshURL mocks base method.
func (m *MockCache) RefreshURL(hash string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshURL", hash)
	ret0, _ := ret[0].(error)
	return ret0
}

// RefreshURL indicates an expected call of RefreshURL.
func (mr *MockCacheMockRecorder) RefreshURL(hash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshURL", reflect.TypeOf((*MockCache)(nil).RefreshURL), hash)
}

// SavePost mocks base method.
func (m *MockCache) SavePost(hash string, post cache.PostRecord) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SavePost", hash, post)
	ret0, _ := ret[0].(error)
	return ret0
}

// SavePost indicates an expected call of SavePost.
func (mr *MockCacheMockRecorder) SavePost(hash, post any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SavePost", reflect.TypeOf((*MockCache)(nil).SavePost), hash, post)
}

// SaveURL mocks base method.
func (m *MockCache) SaveURL(hash string, url cache.URLRecord) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveURL", hash, url)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveURL indicates an expected call of SaveURL.
func (mr *MockCacheMockRecorder) SaveURL(hash, url any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveURL", reflect.TypeOf((*MockCache)(nil).SaveURL), hash, url)
}

// MockStorage is a mock of Storage interface.
type MockStorage struct {
	ctrl     *gomock.Controller
	recorder *MockStorageMockRecorder
	isgomock struct{}
}

// MockStorageMockRecorder is the mock recorder for MockStorage.
type MockStorageMockRecorder struct {
	mock *MockStorage
}

// NewMockStorage creates a new mock instance.
func NewMockStorage(ctrl *gomock.Controller) *MockStorage {
	mock := &MockStorage{ctrl: ctrl}
	mock.recorder = &MockStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStorage) EXPECT() *MockStorageMockRecorder {
	return m.recorder
}

// AddFeedEntry mocks base method.
func (m *MockStorage) AddFeedEntry(entry storage.FeedEntry) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddFeedEntry", entry)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddFeedEntry indicates an expected call of AddFeedEntry.
func (mr *MockStorageMockRecorder) AddFeedEntry(entry any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddFeedEntry", reflect.TypeOf((*MockStorage)(nil).AddFeedEntry), entry)
}

// CleanFeed mocks base method.
func (m *MockStorage) CleanFeed() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CleanFeed")
	ret0, _ := ret[0].(error)
	return ret0
}

// CleanFeed indicates an expected call of CleanFeed.
func (mr *MockStorageMockRecorder) CleanFeed() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CleanFeed", reflect.TypeOf((*MockStorage)(nil).CleanFeed))
}

// FlushEvents mocks base method.
func (m *MockStorage) FlushEvents(start time.Time, events []storage.EventRecord) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FlushEvents", start, events)
	ret0, _ := ret[0].(error)
	return ret0
}

// FlushEvents indicates an expected call of FlushEvents.
func (mr *MockStorageMockRecorder) FlushEvents(start, events any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FlushEvents", reflect.TypeOf((*MockStorage)(nil).FlushEvents), start, events)
}

// GetFeedEntries mocks base method.
func (m *MockStorage) GetFeedEntries() ([]storage.FeedEntry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetFeedEntries")
	ret0, _ := ret[0].([]storage.FeedEntry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetFeedEntries indicates an expected call of GetFeedEntries.
func (mr *MockStorageMockRecorder) GetFeedEntries() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetFeedEntries", reflect.TypeOf((*MockStorage)(nil).GetFeedEntries))
}

// GetThumbnailURL mocks base method.
func (m *MockStorage) GetThumbnailURL(id string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetThumbnailURL", id)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetThumbnailURL indicates an expected call of GetThumbnailURL.
func (mr *MockStorageMockRecorder) GetThumbnailURL(id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetThumbnailURL", reflect.TypeOf((*MockStorage)(nil).GetThumbnailURL), id)
}

// GetURLMetadata mocks base method.
func (m *MockStorage) GetURLMetadata(url string) (storage.URLMetadata, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetURLMetadata", url)
	ret0, _ := ret[0].(storage.URLMetadata)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetURLMetadata indicates an expected call of GetURLMetadata.
func (mr *MockStorageMockRecorder) GetURLMetadata(url any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetURLMetadata", reflect.TypeOf((*MockStorage)(nil).GetURLMetadata), url)
}

// GetURLTranslations mocks base method.
func (m *MockStorage) GetURLTranslations() (map[string]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetURLTranslations")
	ret0, _ := ret[0].(map[string]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetURLTranslations indicates an expected call of GetURLTranslations.
func (mr *MockStorageMockRecorder) GetURLTranslations() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetURLTranslations", reflect.TypeOf((*MockStorage)(nil).GetURLTranslations))
}

// ListEventChunks mocks base method.
func (m *MockStorage) ListEventChunks(start, end time.Time) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListEventChunks", start, end)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListEventChunks indicates an expected call of ListEventChunks.
func (mr *MockStorageMockRecorder) ListEventChunks(start, end any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListEventChunks", reflect.TypeOf((*MockStorage)(nil).ListEventChunks), start, end)
}

// PublishFeeds mocks base method.
func (m *MockStorage) PublishFeeds(atom, json string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishFeeds", atom, json)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishFeeds indicates an expected call of PublishFeeds.
func (mr *MockStorageMockRecorder) PublishFeeds(atom, json any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishFeeds", reflect.TypeOf((*MockStorage)(nil).PublishFeeds), atom, json)
}

// PublishLinkSnapshot mocks base method.
func (m *MockStorage) PublishLinkSnapshot(snapshot []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishLinkSnapshot", snapshot)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishLinkSnapshot indicates an expected call of PublishLinkSnapshot.
func (mr *MockStorageMockRecorder) PublishLinkSnapshot(snapshot any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishLinkSnapshot", reflect.TypeOf((*MockStorage)(nil).PublishLinkSnapshot), snapshot)
}

// PublishSiteSnapshot mocks base method.
func (m *MockStorage) PublishSiteSnapshot(snapshot []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishSiteSnapshot", snapshot)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishSiteSnapshot indicates an expected call of PublishSiteSnapshot.
func (mr *MockStorageMockRecorder) PublishSiteSnapshot(snapshot any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishSiteSnapshot", reflect.TypeOf((*MockStorage)(nil).PublishSiteSnapshot), snapshot)
}

// ReadEvents mocks base method.
func (m *MockStorage) ReadEvents(key string, eventBufferSize int) ([]storage.EventRecord, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadEvents", key, eventBufferSize)
	ret0, _ := ret[0].([]storage.EventRecord)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadEvents indicates an expected call of ReadEvents.
func (mr *MockStorageMockRecorder) ReadEvents(key, eventBufferSize any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadEvents", reflect.TypeOf((*MockStorage)(nil).ReadEvents), key, eventBufferSize)
}

// RecentFeedEntry mocks base method.
func (m *MockStorage) RecentFeedEntry() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RecentFeedEntry")
	ret0, _ := ret[0].(bool)
	return ret0
}

// RecentFeedEntry indicates an expected call of RecentFeedEntry.
func (mr *MockStorageMockRecorder) RecentFeedEntry() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecentFeedEntry", reflect.TypeOf((*MockStorage)(nil).RecentFeedEntry))
}

// SaveThumbnail mocks base method.
func (m *MockStorage) SaveThumbnail(id, url string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveThumbnail", id, url)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SaveThumbnail indicates an expected call of SaveThumbnail.
func (mr *MockStorageMockRecorder) SaveThumbnail(id, url any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveThumbnail", reflect.TypeOf((*MockStorage)(nil).SaveThumbnail), id, url)
}

// SaveURLMetadata mocks base method.
func (m *MockStorage) SaveURLMetadata(metadata storage.URLMetadata) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveURLMetadata", metadata)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveURLMetadata indicates an expected call of SaveURLMetadata.
func (mr *MockStorageMockRecorder) SaveURLMetadata(metadata any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveURLMetadata", reflect.TypeOf((*MockStorage)(nil).SaveURLMetadata), metadata)
}

// SaveURLTranslation mocks base method.
func (m *MockStorage) SaveURLTranslation(translation storage.URLTranslation) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveURLTranslation", translation)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveURLTranslation indicates an expected call of SaveURLTranslation.
func (mr *MockStorageMockRecorder) SaveURLTranslation(translation any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveURLTranslation", reflect.TypeOf((*MockStorage)(nil).SaveURLTranslation), translation)
}

// MockQueue is a mock of Queue interface.
type MockQueue struct {
	ctrl     *gomock.Controller
	recorder *MockQueueMockRecorder
	isgomock struct{}
}

// MockQueueMockRecorder is the mock recorder for MockQueue.
type MockQueueMockRecorder struct {
	mock *MockQueue
}

// NewMockQueue creates a new mock instance.
func NewMockQueue(ctrl *gomock.Controller) *MockQueue {
	mock := &MockQueue{ctrl: ctrl}
	mock.recorder = &MockQueueMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockQueue) EXPECT() *MockQueueMockRecorder {
	return m.recorder
}

// Receive mocks base method.
func (m *MockQueue) Receive() ([]queue.Message, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Receive")
	ret0, _ := ret[0].([]queue.Message)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Receive indicates an expected call of Receive.
func (mr *MockQueueMockRecorder) Receive() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Receive", reflect.TypeOf((*MockQueue)(nil).Receive))
}

// Send mocks base method.
func (m *MockQueue) Send(message queue.Message) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", message)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send.
func (mr *MockQueueMockRecorder) Send(message any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockQueue)(nil).Send), message)
}

// MockBluesky is a mock of Bluesky interface.
type MockBluesky struct {
	ctrl     *gomock.Controller
	recorder *MockBlueskyMockRecorder
	isgomock struct{}
}

// MockBlueskyMockRecorder is the mock recorder for MockBluesky.
type MockBlueskyMockRecorder struct {
	mock *MockBluesky
}

// NewMockBluesky creates a new mock instance.
func NewMockBluesky(ctrl *gomock.Controller) *MockBluesky {
	mock := &MockBluesky{ctrl: ctrl}
	mock.recorder = &MockBlueskyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBluesky) EXPECT() *MockBlueskyMockRecorder {
	return m.recorder
}

// GetPost mocks base method.
func (m *MockBluesky) GetPost(atURI string) (bluesky.Post, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPost", atURI)
	ret0, _ := ret[0].(bluesky.Post)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPost indicates an expected call of GetPost.
func (mr *MockBlueskyMockRecorder) GetPost(atURI any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPost", reflect.TypeOf((*MockBluesky)(nil).GetPost), atURI)
}
