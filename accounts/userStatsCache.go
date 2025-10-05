package accounts

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"sync"
)

type UserStatsCache struct {
	userStatsMap map[string][]byte
	mxState      *sync.RWMutex
	db           *UserStatsService
}

var gSharedUserStatsCache *UserStatsCache

func SharedUserStatsCache(path ...string) *UserStatsCache {
	if gSharedUserStatsCache == nil {
		cacheFilePath := "./cache"
		if len(path) > 0 {
			cacheFilePath = path[0]
		}
		if _, err := os.Stat(cacheFilePath); err != nil {
			os.MkdirAll(cacheFilePath, 0755)
			os.Create(cacheFilePath + "/userStates.db")
		}
		db, err := sql.Open("sqlite", cacheFilePath+"/userStats.db")
		if err != nil {
			panic(err)
		}
		gSharedUserStatsCache = &UserStatsCache{
			userStatsMap: make(map[string][]byte),
			mxState:      new(sync.RWMutex),
			db:           NewUserStatsService(db, "drift_state"),
		}
		gSharedUserStatsCache.init()
	}
	return gSharedUserStatsCache
}

func (p *UserStatsCache) init() {
	rows := p.db.FetchAll()
	if len(rows) == 0 {
		return
	}

	for _, row := range rows {
		contents, err := base64.StdEncoding.DecodeString(row.Data)
		if err != nil || len(contents) != 240 {
			continue
		}
		p.userStatsMap[row.Address] = contents
	}
}
func (p *UserStatsCache) Get(account string, fallback func(string) ([]byte, error)) ([]byte, error) {
	data, exists := p.userStatsMap[account]
	if !exists {
		if fallback != nil {
			var err error
			data, err = fallback(account)
			if err == nil && len(data) > 0 {
				p.userStatsMap[account] = data
				p.saveToDatabase(map[string][]byte{account: data})
			} else {
				return nil, errors.New("Cant load UserStats")
			}
		} else {
			return nil, errors.New("Cant load UserStats")
		}
	} else {
		//data, _ = base64.StdEncoding.DecodeString(row.Data)
	}
	if len(data) != 240 {
		return []byte{}, errors.New("Cant load UserStats")
	}
	return data, nil
}

func (p *UserStatsCache) saveToDatabase(datas map[string][]byte) {
	var dbRows []*UserStatsRecord
	for key, data := range datas {
		dbRows = append(dbRows, &UserStatsRecord{
			Address: key,
			Data:    base64.StdEncoding.EncodeToString(data),
		})
	}
	p.db.Insert(dbRows)
}
