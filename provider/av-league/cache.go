package avleague

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/metatube-community/metatube-sdk-go/model"
)

const (
	cacheDSNConfigKey = "CACHE_DSN"
)

var cacheLogger = log.New(os.Stdout, "[AV-LEAGUE/CACHE]\u0020", log.LstdFlags)

type cachedActor struct {
	ID        string `gorm:"primaryKey;size:64"`
	Payload   []byte `gorm:"not null"`
	UpdatedAt time.Time
}

func (cachedActor) TableName() string { return "actors" }

type cachedImage struct {
	URL         string `gorm:"primaryKey;size:1024"`
	ActorID     string `gorm:"index;size:64"`
	ContentType string `gorm:"size:128"`
	Data        []byte `gorm:"not null"`
	UpdatedAt   time.Time
}

func (cachedImage) TableName() string { return "images" }

type cachedSearch struct {
	Keyword   string `gorm:"primaryKey;size:256"`
	Payload   []byte `gorm:"not null"`
	UpdatedAt time.Time
}

func (cachedSearch) TableName() string { return "searches" }

type actorCache struct {
	db *gorm.DB
	mu sync.RWMutex
}

func defaultCacheDSN() string {
	// Keep cache beside typical local sqlite DB files (same working directory).
	return "av-league-cache.db"
}

func openActorCache(dsn string) (*actorCache, error) {
	if dsn == "" {
		dsn = defaultCacheDSN()
	}
	// Ensure parent directory exists for file-backed SQLite DSNs.
	if !strings.HasPrefix(dsn, "file::memory:") && !strings.Contains(dsn, "mode=memory") {
		path := dsn
		if strings.HasPrefix(path, "file:") {
			path = strings.TrimPrefix(path, "file:")
			if i := strings.IndexByte(path, '?'); i >= 0 {
				path = path[:i]
			}
		}
		if path != "" && path != ":memory:" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, err
			}
		}
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err = db.AutoMigrate(&cachedActor{}, &cachedImage{}, &cachedSearch{}); err != nil {
		return nil, err
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
	cacheLogger.Printf("opened sqlite cache: %s", dsn)
	return &actorCache{db: db}, nil
}

func (c *actorCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func normalizeSearchKeyword(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

func (c *actorCache) getActor(id string) (*model.ActorInfo, bool) {
	if c == nil || id == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var row cachedActor
	if err := c.db.Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cacheLogger.Printf("actor miss: id=%s", id)
		} else {
			cacheLogger.Printf("actor miss: id=%s err=%v", id, err)
		}
		return nil, false
	}
	info := &model.ActorInfo{}
	if err := json.Unmarshal(row.Payload, info); err != nil || !info.IsValid() {
		cacheLogger.Printf("actor miss: id=%s invalid payload err=%v", id, err)
		return nil, false
	}
	cacheLogger.Printf("actor hit: id=%s name=%s", info.ID, info.Name)
	return info, true
}

func (c *actorCache) putActor(info *model.ActorInfo) error {
	if c == nil || info == nil || !info.IsValid() {
		return nil
	}
	payload, err := json.Marshal(info)
	if err != nil {
		cacheLogger.Printf("actor store failed: id=%s err=%v", info.ID, err)
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cachedActor{
		ID:      info.ID,
		Payload: payload,
	}).Error; err != nil {
		cacheLogger.Printf("actor store failed: id=%s err=%v", info.ID, err)
		return err
	}
	cacheLogger.Printf("actor stored: id=%s name=%s images=%d", info.ID, info.Name, len(info.Images))
	return nil
}

func (c *actorCache) getSearch(keyword string) ([]*model.ActorSearchResult, bool) {
	if c == nil {
		return nil, false
	}
	key := normalizeSearchKeyword(keyword)
	if key == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var row cachedSearch
	if err := c.db.Where("keyword = ?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cacheLogger.Printf("search miss: keyword=%q", key)
		} else {
			cacheLogger.Printf("search miss: keyword=%q err=%v", key, err)
		}
		return nil, false
	}
	var results []*model.ActorSearchResult
	if err := json.Unmarshal(row.Payload, &results); err != nil || len(results) == 0 {
		cacheLogger.Printf("search miss: keyword=%q invalid payload err=%v", key, err)
		return nil, false
	}
	cacheLogger.Printf("search hit: keyword=%q results=%d", key, len(results))
	return results, true
}

func (c *actorCache) putSearch(keyword string, results []*model.ActorSearchResult) error {
	if c == nil || len(results) == 0 {
		return nil
	}
	key := normalizeSearchKeyword(keyword)
	if key == "" {
		return nil
	}
	payload, err := json.Marshal(results)
	if err != nil {
		cacheLogger.Printf("search store failed: keyword=%q err=%v", key, err)
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cachedSearch{
		Keyword: key,
		Payload: payload,
	}).Error; err != nil {
		cacheLogger.Printf("search store failed: keyword=%q err=%v", key, err)
		return err
	}
	cacheLogger.Printf("search stored: keyword=%q results=%d", key, len(results))
	return nil
}

func (c *actorCache) getImage(rawURL string) (data []byte, contentType string, ok bool) {
	if c == nil || rawURL == "" {
		return nil, "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var row cachedImage
	if err := c.db.Where("url = ?", rawURL).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cacheLogger.Printf("image miss: url=%s", rawURL)
		} else {
			cacheLogger.Printf("image miss: url=%s err=%v", rawURL, err)
		}
		return nil, "", false
	}
	if len(row.Data) == 0 {
		cacheLogger.Printf("image miss: url=%s empty blob", rawURL)
		return nil, "", false
	}
	cacheLogger.Printf("image hit: url=%s size=%d content_type=%s", rawURL, len(row.Data), row.ContentType)
	return row.Data, row.ContentType, true
}

func (c *actorCache) putImage(rawURL, actorID, contentType string, data []byte) error {
	if c == nil || rawURL == "" || len(data) == 0 {
		return nil
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&cachedImage{
		URL:         rawURL,
		ActorID:     actorID,
		ContentType: contentType,
		Data:        data,
	}).Error; err != nil {
		cacheLogger.Printf("image store failed: url=%s actor_id=%s err=%v", rawURL, actorID, err)
		return err
	}
	cacheLogger.Printf("image stored: url=%s actor_id=%s size=%d content_type=%s", rawURL, actorID, len(data), contentType)
	return nil
}
