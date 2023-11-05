package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// 参加者向けAPI
// GET /api/player/player/:player_id
// 参加者の詳細情報を取得する
func playerHandler(c echo.Context) error {
	ctx := context.Background()

	v, err := parseViewer(c)
	if err != nil {
		return err
	}
	if v.role != RolePlayer {
		return echo.NewHTTPError(http.StatusForbidden, "role player required")
	}

	tenantDB, err := connectToTenantDB(v.tenantID)
	if err != nil {
		return err
	}
	defer tenantDB.Close()

	if err := authorizePlayer(ctx, tenantDB, v.playerID); err != nil {
		return err
	}

	playerID := c.Param("player_id")
	if playerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "player_id is required")
	}
	p, err := retrievePlayer(ctx, tenantDB, playerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "player not found")
		}
		return fmt.Errorf("error retrievePlayer: %w", err)
	}
	cs := []CompetitionRow{}
	if err := tenantDB.SelectContext(
		ctx,
		&cs,
		"SELECT * FROM competition WHERE tenant_id = ? ORDER BY created_at ASC",
		v.tenantID,
	); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("error Select competition: %w", err)
	}

	// player_scoreを読んでいるときに更新が走ると不整合が起こるのでロックを取得する
	fl, err := flockByTenantID(v.tenantID)
	if err != nil {
		return fmt.Errorf("error flockByTenantID: %w", err)
	}
	defer fl.Close()
	pss := make([]PlayerScoreRow, 0, len(cs))
	for _, c := range cs {
		ps := PlayerScoreRow{}
		if err := tenantDB.GetContext(
			ctx,
			&ps,
			// 最後にCSVに登場したスコアを採用する = row_numが一番大きいもの
			"SELECT * FROM player_score WHERE tenant_id = ? AND competition_id = ? AND player_id = ? ORDER BY row_num DESC LIMIT 1",
			v.tenantID,
			c.ID,
			p.ID,
		); err != nil {
			// 行がない = スコアが記録されてない
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return fmt.Errorf("error Select player_score: tenantID=%d, competitionID=%s, playerID=%s, %w", v.tenantID, c.ID, p.ID, err)
		}
		pss = append(pss, ps)
	}

	psds := make([]PlayerScoreDetail, 0, len(pss))

	competitionIds := make([]string, 0, len(pss))
	for _, ps := range pss {
		competitionIds = append(competitionIds, ps.CompetitionID)
	}
	competitions, err := retrieveCompetitions(ctx, tenantDB, competitionIds)
	if err != nil {
		return fmt.Errorf("error retrieveCompetitions: %w", err)
	}
	for _, ps := range pss {
		for _, comp := range *competitions {
			if ps.CompetitionID == comp.ID {
				psds = append(psds, PlayerScoreDetail{
					CompetitionTitle: comp.Title,
					Score:            ps.Score,
				})
			}
		}
	}

	res := SuccessResult{
		Status: true,
		Data: PlayerHandlerResult{
			Player: PlayerDetail{
				ID:             p.ID,
				DisplayName:    p.DisplayName,
				IsDisqualified: p.IsDisqualified,
			},
			Scores: psds,
		},
	}
	return c.JSON(http.StatusOK, res)
}
