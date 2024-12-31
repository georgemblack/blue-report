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

	cache "github.com/georgemblack/blue-report/pkg/cache"
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

// PublishSite mocks base method.
func (m *MockStorage) PublishSite(site []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublishSite", site)
	ret0, _ := ret[0].(error)
	return ret0
}

// PublishSite indicates an expected call of PublishSite.
func (mr *MockStorageMockRecorder) PublishSite(site any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublishSite", reflect.TypeOf((*MockStorage)(nil).PublishSite), site)
}

// ReadEvents mocks base method.
func (m *MockStorage) ReadEvents(key string) ([]storage.EventRecord, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReadEvents", key)
	ret0, _ := ret[0].([]storage.EventRecord)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReadEvents indicates an expected call of ReadEvents.
func (mr *MockStorageMockRecorder) ReadEvents(key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReadEvents", reflect.TypeOf((*MockStorage)(nil).ReadEvents), key)
}

// SaveThumbnail mocks base method.
func (m *MockStorage) SaveThumbnail(id, url string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SaveThumbnail", id, url)
	ret0, _ := ret[0].(error)
	return ret0
}

// SaveThumbnail indicates an expected call of SaveThumbnail.
func (mr *MockStorageMockRecorder) SaveThumbnail(id, url any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SaveThumbnail", reflect.TypeOf((*MockStorage)(nil).SaveThumbnail), id, url)
}
