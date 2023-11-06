package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// テナント管理者向けAPI
// POST /api/organizer/competition/:competition_id/score
// 大会のスコアをCSVでアップロードする
func competitionScoreHandler(c echo.Context) error {
	ctx := context.Background()
	v, err := parseViewer(c)
	if err != nil {
		return fmt.Errorf("error parseViewer: %w", err)
	}
	if v.role != RoleOrganizer {
		return echo.NewHTTPError(http.StatusForbidden, "role organizer required")
	}

	tenantDB, err := connectToTenantDB(v.tenantID)
	if err != nil {
		return err
	}
	defer tenantDB.Close()

	competitionID := c.Param("competition_id")
	if competitionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "competition_id required")
	}
	comp, err := retrieveCompetition(ctx, tenantDB, competitionID)
	if err != nil {
		// 存在しない大会
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "competition not found")
		}
		return fmt.Errorf("error retrieveCompetition: %w", err)
	}
	if comp.FinishedAt.Valid {
		res := FailureResult{
			Status:  false,
			Message: "competition is finished",
		}
		return c.JSON(http.StatusBadRequest, res)
	}

	fh, err := c.FormFile("scores")
	if err != nil {
		return fmt.Errorf("error c.FormFile(scores): %w", err)
	}
	f, err := fh.Open()
	if err != nil {
		return fmt.Errorf("error fh.Open FormFile(scores): %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	headers, err := r.Read()
	if err != nil {
		return fmt.Errorf("error r.Read at header: %w", err)
	}
	if !reflect.DeepEqual(headers, []string{"player_id", "score"}) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid CSV headers")
	}

	// / DELETEしたタイミングで参照が来ると空っぽのランキングになるのでロックする
	fl, err := flockByTenantID(v.tenantID)
	if err != nil {
		return fmt.Errorf("error flockByTenantID: %w", err)
	}
	defer fl.Close()
	var rowNum int64
	playerScoreRows := []PlayerScoreRow{}
	for {
		rowNum++
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error r.Read at rows: %w", err)
		}
		if len(row) != 2 {
			return fmt.Errorf("row must have two columns: %#v", row)
		}
		playerID, scoreStr := row[0], row[1]
		if _, err := retrievePlayer(ctx, tenantDB, playerID); err != nil {
			// 存在しない参加者が含まれている
			if errors.Is(err, sql.ErrNoRows) {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprintf("player not found: %s", playerID),
				)
			}
			return fmt.Errorf("error retrievePlayer: %w", err)
		}
		var score int64
		if score, err = strconv.ParseInt(scoreStr, 10, 64); err != nil {
			return echo.NewHTTPError(
				http.StatusBadRequest,
				fmt.Sprintf("error strconv.ParseUint: scoreStr=%s, %s", scoreStr, err),
			)
		}
		id, err := dispenseID(ctx)
		if err != nil {
			return fmt.Errorf("error dispenseID: %w", err)
		}
		now := time.Now().Unix()
		playerScoreRows = append(playerScoreRows, PlayerScoreRow{
			ID:            id,
			TenantID:      v.tenantID,
			PlayerID:      playerID,
			CompetitionID: competitionID,
			Score:         score,
			RowNum:        rowNum,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	if _, err := tenantDB.ExecContext(
		ctx,
		"DELETE FROM player_score WHERE tenant_id = ? AND competition_id = ?",
		v.tenantID,
		competitionID,
	); err != nil {
		return fmt.Errorf("error Delete player_score: tenantID=%d, competitionID=%s, %w", v.tenantID, competitionID, err)
	}
	for _, ps := range playerScoreRows {
		if _, err := tenantDB.NamedExecContext(
			ctx,
			"INSERT INTO player_score (id, tenant_id, player_id, competition_id, score, row_num, created_at, updated_at) VALUES (:id, :tenant_id, :player_id, :competition_id, :score, :row_num, :created_at, :updated_at)",
			ps,
		); err != nil {
			return fmt.Errorf(
				"error Insert player_score: id=%s, tenant_id=%d, playerID=%s, competitionID=%s, score=%d, rowNum=%d, createdAt=%d, updatedAt=%d, %w",
				ps.ID, ps.TenantID, ps.PlayerID, ps.CompetitionID, ps.Score, ps.RowNum, ps.CreatedAt, ps.UpdatedAt, err,
			)
		}
	}

	err = c.JSON(http.StatusOK, SuccessResult{
		Status: true,
		Data:   ScoreHandlerResult{Rows: int64(len(playerScoreRows))},
	})
	if err != nil {
		return fmt.Errorf("error c.JSON: %w", err)
	}

	err = RefreshRanking(ctx, tenantDB, v.tenantID, competitionID)
	if err != nil {
		return fmt.Errorf("error RefreshRanking: %w", err)
	}

	return nil
}
