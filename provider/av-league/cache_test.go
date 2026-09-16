package avleague

import (
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metatube-community/metatube-sdk-go/model"
)

func TestDefaultCacheDSN_BesideMainDSN(t *testing.T) {
	t.Setenv("DSN", "/config/metatube.db")
	assert.Equal(t, filepath.Join("/config", "av-league-cache.db"), defaultCacheDSN())

	t.Setenv("DSN", "file:/data/metatube.db?cache=shared")
	assert.Equal(t, filepath.Join("/data", "av-league-cache.db"), defaultCacheDSN())

	t.Setenv("DSN", "postgres://u:p@localhost/db")
	assert.Equal(t, "av-league-cache.db", defaultCacheDSN())

	t.Setenv("DSN", "")
	assert.Equal(t, "av-league-cache.db", defaultCacheDSN())
}

func newTestCache(t *testing.T) *actorCache {
	t.Helper()
	cache, err := openActorCache(filepath.Join(t.TempDir(), "cache.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cache.Close() })
	return cache
}

func TestActorCache_ActorAndImage(t *testing.T) {
	cache := newTestCache(t)

	info := &model.ActorInfo{
		ID:       "8301",
		Name:     "白川ゆず",
		Provider: Name,
		Homepage: "https://www.av-league.com/actress/8301.html",
		Images:   []string{"https://www.av-league.com/img/8301.jpg"},
		Aliases:  []string{},
	}
	require.NoError(t, cache.putActor(info))
	require.NoError(t, cache.putImage(info.Images[0], info.ID, "image/jpeg", []byte("fake-jpeg")))

	got, ok := cache.getActor("8301")
	require.True(t, ok)
	assert.Equal(t, info.Name, got.Name)
	assert.Equal(t, []string(info.Images), []string(got.Images))

	data, ct, ok := cache.getImage(info.Images[0])
	require.True(t, ok)
	assert.Equal(t, "image/jpeg", ct)
	assert.Equal(t, []byte("fake-jpeg"), data)
}

func TestActorCache_Search(t *testing.T) {
	cache := newTestCache(t)

	results := []*model.ActorSearchResult{{
		ID:       "8301",
		Name:     "白川ゆず",
		Provider: Name,
		Homepage: "https://www.av-league.com/actress/8301.html",
		Images:   []string{"https://www.av-league.com/img/8301.jpg"},
	}}
	require.NoError(t, cache.putSearch("白川ゆず", results))

	got, ok := cache.getSearch(" 白川ゆず ")
	require.True(t, ok)
	require.Len(t, got, 1)
	assert.Equal(t, "8301", got[0].ID)
}

func TestAVLeague_CacheHitSkipsRemote(t *testing.T) {
	cache := newTestCache(t)

	info := &model.ActorInfo{
		ID:       "8301",
		Name:     "白川ゆず",
		Provider: Name,
		Homepage: "https://www.av-league.com/actress/8301.html",
		Images:   []string{"https://example.com/a.jpg"},
		Aliases:  []string{},
	}
	require.NoError(t, cache.putActor(info))
	require.NoError(t, cache.putImage(info.Images[0], info.ID, "image/jpeg", []byte{0xff, 0xd8, 0xff}))

	avl := New()
	if avl.cache != nil {
		_ = avl.cache.Close()
	}
	avl.cache = cache

	got, err := avl.GetActorInfoByID("8301")
	require.NoError(t, err)
	assert.Equal(t, "白川ゆず", got.Name)

	resp, err := avl.Fetch(info.Images[0])
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xff, 0xd8, 0xff}, body)
	assert.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"))
}

func TestAVLeague_SetConfigCacheDSN(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "custom.db")
	avl := New()
	if avl.cache != nil {
		_ = avl.cache.Close()
	}
	require.NoError(t, avl.SetConfig(mapConfig{cacheDSNConfigKey: dsn}))
	t.Cleanup(func() { _ = avl.cache.Close() })

	info := &model.ActorInfo{
		ID:       "1",
		Name:     "test",
		Provider: Name,
		Homepage: "https://www.av-league.com/actress/1.html",
		Images:   []string{},
		Aliases:  []string{},
	}
	require.NoError(t, avl.cache.putActor(info))

	got, ok := avl.cache.getActor("1")
	require.True(t, ok)
	assert.Equal(t, "test", got.Name)
}

type mapConfig map[string]string

func (m mapConfig) Has(key string) bool {
	_, ok := m[key]
	return ok
}

func (m mapConfig) GetString(key string) (string, error) { return m[key], nil }
func (m mapConfig) GetBool(string) (bool, error)         { return false, nil }
func (m mapConfig) GetInt64(string) (int64, error)       { return 0, nil }
func (m mapConfig) GetFloat64(string) (float64, error)   { return 0, nil }
func (m mapConfig) GetDuration(string) (time.Duration, error) {
	return 0, nil
}
