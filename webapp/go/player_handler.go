package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
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

	if _, err := authorizePlayer(ctx, tenantDB, v.playerID); err != nil {
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

	pss := make([]PlayerScoreRow, 0, len(cs))

	baseSql := `
SELECT id, tenant_id, player_id, competition_id, score, MAX(row_num) row_num, created_at, updated_at
FROM player_score
WHERE tenant_id = ? AND player_id = ?
GROUP BY competition_id
`
	sql, args, err := sqlx.In(baseSql, v.tenantID, p.ID)
	if err != nil {
		return fmt.Errorf("error sqlx.In: %w", err)
	}
	if err := tenantDB.SelectContext(ctx, &pss, sql, args...); err != nil {
		return fmt.Errorf("error Select player_score: %w", err)
	}

	psds := make([]PlayerScoreDetail, 0, len(pss))

	if err != nil {
		return fmt.Errorf("error retrieveCompetitions: %w", err)
	}
	for _, ps := range pss {
		for _, comp := range cs {
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
