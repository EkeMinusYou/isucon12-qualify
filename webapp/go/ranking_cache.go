package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
)

type rankingCacheSlice struct {
	sync.RWMutex
	data map[string][]PlayerScoreRowWithPlayerRow
}

func NewRankingCacheSlice() *rankingCacheSlice {
	return &rankingCacheSlice{
		data: make(map[string][]PlayerScoreRowWithPlayerRow),
	}
}

func (c *rankingCacheSlice) Set(tenantID int64, competitionID string, value []PlayerScoreRowWithPlayerRow) {
	c.Lock()
	defer c.Unlock()
	c.data[fmt.Sprintf("%d:%s", tenantID, competitionID)] = value
}

func (c *rankingCacheSlice) Get(tenantID int64, competitionID string) ([]PlayerScoreRowWithPlayerRow, bool) {
	c.RLock()
	defer c.RUnlock()
	v, ok := c.data[fmt.Sprintf("%d:%s", tenantID, competitionID)]
	return v, ok
}

var RankingCache = NewRankingCacheSlice()

func RefreshRanking(ctx context.Context, tenantDB *sqlx.DB, tenantID int64, competitionID string) error {
	pss := []PlayerScoreRowWithPlayerRow{}
	if err := tenantDB.SelectContext(
		ctx,
		&pss,
		`SELECT
      ps.id AS "ps.id",
      ps.tenant_id AS "ps.tenant_id",
      ps.player_id AS "ps.player_id",
      ps.competition_id AS "ps.competition_id",
      ps.score AS "ps.score",
      ps.row_num AS "ps.row_num",
      ps.created_at AS "ps.created_at",
      ps.updated_at AS "ps.updated_at",
      p.id AS "p.id",
      p.tenant_id AS "p.tenant_id",
      p.display_name AS "p.display_name",
      p.is_disqualified AS "p.is_disqualified",
      p.created_at AS "p.created_at"
    FROM player_score ps INNER JOIN player p ON ps.player_id = p.id WHERE ps.tenant_id = ? AND ps.competition_id = ? ORDER BY ps.row_num DESC`,
		tenantID,
		competitionID,
	); err != nil {
		return fmt.Errorf("error Select player_score: tenantID=%d, competitionID=%s, %w", tenantID, competitionID, err)
	}

	RankingCache.Set(tenantID, competitionID, pss)

	return nil
}
