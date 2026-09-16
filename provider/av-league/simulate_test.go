package avleague

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAVLeague_SimulateRealRequests simulates the real client flow:
// search -> actor info -> fetch images, then repeat against the same SQLite cache.
// Round 1 may hit av-league.com; round 2 must be served from local cache
// (verified by opening a fresh provider on the same CACHE_DSN).
func TestAVLeague_SimulateRealRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skip network simulation in short mode")
	}

	dsn := filepath.Join(t.TempDir(), "av-league-sim.db")
	keyword := "白川ゆず"

	avl := New()
	if avl.cache != nil {
		_ = avl.cache.Close()
	}
	require.NoError(t, avl.SetConfig(mapConfig{cacheDSNConfigKey: dsn}))
	t.Cleanup(func() { _ = avl.cache.Close() })

	t.Log("round 1: search / info / images (may hit remote)")
	results, err := avl.SearchActor(keyword)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	t.Logf("search hit remote/cache: count=%d first=%s id=%s", len(results), results[0].Name, results[0].ID)

	actorID := results[0].ID
	info, err := avl.GetActorInfoByID(actorID)
	require.NoError(t, err)
	require.True(t, info.IsValid())
	t.Logf("actor info: id=%s name=%s images=%d", info.ID, info.Name, len(info.Images))

	for i, imageURL := range info.Images {
		resp, err := avl.Fetch(imageURL)
		require.NoError(t, err)
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		require.NotEmpty(t, data)
		t.Logf("fetch image #%d: status=%d size=%d content_type=%s", i, resp.StatusCode, len(data), resp.Header.Get("Content-Type"))
	}

	_, ok := avl.cache.getSearch(keyword)
	require.True(t, ok, "search should be cached after round 1")
	_, ok = avl.cache.getActor(actorID)
	require.True(t, ok, "actor should be cached after round 1")
	for _, imageURL := range info.Images {
		_, _, ok = avl.cache.getImage(imageURL)
		require.True(t, ok, "image should be cached: %s", imageURL)
	}

	t.Log("round 2: fresh provider + same sqlite dsn (must not need remote for metadata)")
	avl2 := New()
	if avl2.cache != nil {
		_ = avl2.cache.Close()
	}
	require.NoError(t, avl2.SetConfig(mapConfig{cacheDSNConfigKey: dsn}))
	t.Cleanup(func() { _ = avl2.cache.Close() })

	start := time.Now()
	results2, err := avl2.SearchActor(keyword)
	searchElapsed := time.Since(start)
	require.NoError(t, err)
	require.NotEmpty(t, results2)
	assert.Equal(t, actorID, results2[0].ID)
	t.Logf("cached search: count=%d elapsed=%s", len(results2), searchElapsed)

	start = time.Now()
	info2, err := avl2.GetActorInfoByID(actorID)
	infoElapsed := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, info.Name, info2.Name)
	t.Logf("cached actor info: name=%s elapsed=%s", info2.Name, infoElapsed)

	for i, imageURL := range info2.Images {
		start = time.Now()
		resp, err := avl2.Fetch(imageURL)
		elapsed := time.Since(start)
		require.NoError(t, err)
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		require.NotEmpty(t, data)
		t.Logf("cached image #%d: size=%d elapsed=%s", i, len(data), elapsed)
	}
}

// TestAVLeague_SimulateHTTPAPI hits a running MetaTube HTTP API the same way a client would.
//
//	MT_BASE_URL=http://127.0.0.1:8080 go test ./provider/av-league -run TestAVLeague_SimulateHTTPAPI -v
//	MT_TOKEN=xxx MT_BASE_URL=http://127.0.0.1:8080 go test ./provider/av-league -run TestAVLeague_SimulateHTTPAPI -v
func TestAVLeague_SimulateHTTPAPI(t *testing.T) {
	baseURL := os.Getenv("MT_BASE_URL")
	if baseURL == "" {
		t.Skip("set MT_BASE_URL to simulate against a live server, e.g. MT_BASE_URL=http://127.0.0.1:8080")
	}
	token := os.Getenv("MT_TOKEN")
	keyword := envOr("MT_ACTOR_KEYWORD", "白川ゆず")
	provider := envOr("MT_ACTOR_PROVIDER", Name)
	rounds := 2

	client := &http.Client{Timeout: 2 * time.Minute}
	doGET := func(name, rawURL string) (status int, body []byte, elapsed time.Duration) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "metatube-av-league-simulate-test/1.0")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		start := time.Now()
		resp, err := client.Do(req)
		elapsed = time.Since(start)
		require.NoError(t, err, name)
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err, name)
		status = resp.StatusCode
		t.Logf("%s => %d in %s url=%s", name, status, elapsed, rawURL)
		if len(body) > 0 && len(body) < 500 {
			t.Logf("body: %s", string(body))
		} else if len(body) >= 500 {
			t.Logf("body: %s...", string(body[:500]))
		}
		return
	}

	// health
	status, _, _ := doGET("index", stringsTrimRightSlash(baseURL)+"/")
	require.Equal(t, http.StatusOK, status)

	var actorID string
	searchURL := fmt.Sprintf("%s/v1/actors/search?q=%s&provider=%s&fallback=true",
		stringsTrimRightSlash(baseURL),
		url.QueryEscape(keyword),
		url.QueryEscape(provider),
	)

	for i := 1; i <= rounds; i++ {
		status, body, _ := doGET(fmt.Sprintf("actor search #%d", i), searchURL)
		require.Equal(t, http.StatusOK, status)
		var payload struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &payload))
		require.NotEmpty(t, payload.Data)
		if actorID == "" {
			actorID = payload.Data[0].ID
			t.Logf("resolved actor id=%s name=%s", actorID, payload.Data[0].Name)
		}
	}

	infoURL := fmt.Sprintf("%s/v1/actors/%s/%s?lazy=true",
		stringsTrimRightSlash(baseURL),
		url.PathEscape(provider),
		url.PathEscape(actorID),
	)
	for i := 1; i <= rounds; i++ {
		status, body, _ := doGET(fmt.Sprintf("actor info #%d", i), infoURL)
		require.Equal(t, http.StatusOK, status)
		require.Contains(t, string(body), actorID)
	}

	imageURL := fmt.Sprintf("%s/v1/images/primary/%s/%s",
		stringsTrimRightSlash(baseURL),
		url.PathEscape(provider),
		url.PathEscape(actorID),
	)
	for i := 1; i <= rounds; i++ {
		status, body, _ := doGET(fmt.Sprintf("actor primary image #%d", i), imageURL)
		require.Equal(t, http.StatusOK, status)
		require.NotEmpty(t, body)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
