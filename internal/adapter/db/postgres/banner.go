package db

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/The-Gleb/banner_service/pkg/client/postgresql"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type bannerStorage struct {
	client postgresql.Client
}

func NewBannerStorage(client postgresql.Client) *bannerStorage {
	return &bannerStorage{client: client}
}

func (s *bannerStorage) CreateBanner(ctx context.Context, dto entity.CreateBannerDTO) (int64, error) {

	tx, err := s.client.Begin(ctx)
	if err != nil {
		slog.Error("error beginnig transaction",
			"error", err,
		)
		return 0, errors.NewDomainError(errors.ErrDB, "")
	}
	defer tx.Rollback(ctx)

	unique, err := isUnique(ctx, tx, dto.TagIDs, dto.FeatureID)
	if err != nil {
		return 0, errors.NewDomainError(errors.ErrDB, "")
	}

	if !unique {
		return 0, errors.NewDomainError(errors.ErrAlreadyExists, "banner with these tags and feature already exists")
	}

	query := fmt.Sprintf(
		`INSERT INTO
			banners ("title", "text", "url", "is_active", "created_at")
		VALUES
			(%s, %s, %s, %t, NOW();`,
		dto.Content.Title, dto.Content.Text, dto.Content.URL, dto.IsActive,
	)

	row := s.client.QueryRow(ctx, query)

	var bannerID int64
	err = row.Scan(&bannerID)
	if err != nil {
		slog.Error("error scanning from row",
			"error", err,
		)
		return 0, errors.NewDomainError(errors.ErrDB, "")
	}

	tagsString := "" // TODO:
	for _, tagID := range dto.TagIDs {
		tagsString += fmt.Sprintf("(%d, %d),", bannerID, tagID)
	}
	tagsString = strings.TrimSuffix(tagsString, ",")

	query = fmt.Sprintf(
		`INSERT INTO
			banner_tag ("banner_id", "tag_id)
		VALUES
			%s;`,
		tagsString,
	)

	_, err = s.client.Exec(ctx, query)
	if err != nil {
		slog.Error("error inserting in banner_tag",
			"error", err,
		)
		var pgErr *pgconn.PgError
		if stdErrors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return 0, errors.NewDomainError(errors.ErrTagNotFound, "")
		}
		return 0, errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`INSERT INTO
			banner_feature ("banner_id", "feature_id)
		VALUES
			(%d, %d);`,
		bannerID, dto.FeatureID,
	)

	_, err = s.client.Exec(ctx, query)
	if err != nil {
		slog.Error("error inserting in banner_feature",
			"error", err,
		)
		var pgErr *pgconn.PgError
		if stdErrors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return 0, errors.NewDomainError(errors.ErrFeatureNotFound, "")
		}
		return 0, errors.NewDomainError(errors.ErrDB, "")
	}

	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("error commiting transaction",
			"error", err,
		)
		return 0, errors.NewDomainError(errors.ErrDB, "")
	}

	return bannerID, nil
}

func (s *bannerStorage) DeleteBanner(ctx context.Context, dto entity.DeleteBannerDTO) error {

	c, err := s.client.Exec(
		ctx,
		`DELETE FROM banners CASCADE
		WHERE id = $1;`,
		dto.BannerID,
	)
	if err != nil {
		slog.Error("error deleting from banners",
			"error", err,
		)
		return errors.NewDomainError(errors.ErrDB, "")
	}
	if c.RowsAffected() == 0 {
		slog.Error("error deleting from banner_tag, id not found")
		return errors.NewDomainError(errors.ErrNoDataFound, "")
	}

	return nil

}

func (s *bannerStorage) GetUserBanner(ctx context.Context, dto entity.GetUserBannerDTO) (entity.UpdateCacheDTO, error) {

	tx, err := s.client.Begin(ctx)
	if err != nil {
		slog.Error("error beginnig transaction",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(
		`SELECT
			banner_id
		FROM
			banner_tag
		GROUP BY
			banner_id
		HAVING
			tag_id = %d;`,
		dto.TagID,
	)

	rows, err := tx.Query(ctx, query)
	if err != nil {
		slog.Error("error selecting from banner_tag",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDs, err := pgx.CollectRows[int64](rows, func(row pgx.CollectableRow) (int64, error) {
		var id int64
		err := row.Scan(&id)
		return id, err
	})

	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDsString := fmt.Sprint(bannerIDs) // TODO:

	query = fmt.Sprintf(
		`SELECT banner_id
		FROM banner_feature
		WHERE banner_id IN %s AND feature_id = %d;`,
		bannerIDsString, dto.FeatureID,
	)

	row := tx.QueryRow(ctx, query)

	var bannerID int64
	err = row.Scan(&bannerID)
	if err != nil {
		slog.Error("error scanning row",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`SELECT title, text, url, is_active
		FROM banners
		WHERE id = %d;`,
		bannerID,
	)

	row = tx.QueryRow(ctx, query)

	var isActive bool
	content := entity.BannerContent{}
	err = row.Scan(&content.Title, &content.Text, &content.URL, &isActive)
	if err != nil {
		slog.Error("error scanning row",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	if !isActive && !dto.IsAdmin {
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrForbidden, "")
	}

	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("error commiting transaction",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	return entity.UpdateCacheDTO{
		BannerID:  bannerID,
		Content:   content,
		TagID:     dto.TagID,
		FeatureID: dto.FeatureID,
		IsActive:  isActive,
		IsAdmin:   dto.IsAdmin,
	}, nil

}

func (s *bannerStorage) GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.BannerContent, error) {

	tx, err := s.client.Begin(ctx)
	if err != nil {
		slog.Error("error beginnig transaction",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}
	defer tx.Rollback(ctx)

	tagID, tagOK := dto.Filters["tag"]
	featureID, featureOK := dto.Filters["feature"]

	query := `SELECT
			banner_id
		FROM
			banner_tag JOIN banner_feature
			ON banner_tag.banner_id = banner_feature.banner_id
		`

	if featureOK {
		query = fmt.Sprintf(
			`%s
			WHERE feature_id = %d`,
			query, featureID,
		)
	}

	query += "GROUP BY banner_id, feature_id"

	if tagOK {
		query = fmt.Sprintf(
			`%s
			HAVING
				tag_id = %d`,
			query, tagID,
		)
	}

	query = fmt.Sprintf(
		`%s
		LIMIT %d
		OFFSET %d;`,
		query, dto.Limit, dto.Offset,
	)

	slog.Debug("query", "filters", dto.Filters, "query", query)

	rows, err := tx.Query(ctx, query)
	if err != nil {
		slog.Error("error getting filtered ids from db",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDs, err := pgx.CollectRows[int64](rows, func(row pgx.CollectableRow) (int64, error) {
		var id int64
		err := row.Scan(&id)
		return id, err
	})
	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDsString := fmt.Sprint(bannerIDs)

	query = fmt.Sprintf(
		`SELECT title, text, url
		FROM banners
		WHERE id IN %s;`,
		bannerIDsString,
	)

	rows, err = tx.Query(ctx, query)
	if err != nil {
		slog.Error("error selecting banners content from db",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	bannersContent, err := pgx.CollectRows[entity.BannerContent](rows, func(row pgx.CollectableRow) (entity.BannerContent, error) {
		var c entity.BannerContent
		err := row.Scan(&c.Title, &c.Text, &c.URL)
		return c, err
	})
	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("error commiting transaction",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	return bannersContent, nil
}

func (s *bannerStorage) UpdateBanner(ctx context.Context, dto entity.UpdateBannerDTO) error {

	tx, err := s.client.Begin(ctx)
	if err != nil {
		slog.Error("error beginnig transaction",
			"error", err,
		)
		return errors.NewDomainError(errors.ErrDB, "")
	}
	defer tx.Rollback(ctx)

	unique, err := isUnique(ctx, tx, dto.TagIDs, dto.FeatureID)
	if err != nil {
		return errors.NewDomainError(errors.ErrDB, "")
	}

	if !unique {
		return errors.NewDomainError(errors.ErrAlreadyExists, "banner with these tags and feature already exists")
	}

	c, err := tx.Exec(
		ctx,
		`DELETE FROM banner_tag
		WHERE banner_id = $1;`,
		dto.BannerID,
	)

	if err != nil {
		slog.Error("error updating banner_tag",
			"error", err,
		)
		return errors.NewDomainError(errors.ErrDB, "")
	}
	if c.RowsAffected() == 0 {
		slog.Error("error updating banner_tag, id not found")
		return errors.NewDomainError(errors.ErrNoDataFound, "")
	}

	tagsString := "" // TODO:
	for _, tagID := range dto.TagIDs {
		tagsString += fmt.Sprintf("(%d, %d),", dto.BannerID, tagID)
	}
	tagsString = strings.TrimSuffix(tagsString, ",")

	query := fmt.Sprintf(
		`INSERT INTO
			banner_tag ("banner_id", "tag_id)
		VALUES
			%s;`,
		tagsString,
	)

	_, err = s.client.Exec(ctx, query)
	if err != nil {
		slog.Error("error inserting in banner_tag",
			"error", err,
		)
		var pgErr *pgconn.PgError
		if stdErrors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return errors.NewDomainError(errors.ErrTagNotFound, "")
		}
		return errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`UPDATE banner_feature
		SET feature_id = %d
		WHERE banner_id = %d;`,
		dto.FeatureID, dto.BannerID,
	)

	_, err = s.client.Exec(ctx, query)
	if err != nil {
		slog.Error("error updating banner_feature",
			"error", err,
		)
		var pgErr *pgconn.PgError
		if stdErrors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return errors.NewDomainError(errors.ErrTagNotFound, "")
		}
		return errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`UPDATE banners
		SET
			title = %s, text = %s,
			url = %s, is_active = %t, updated_at = NOW() 
		WHERE banner_id = %d;`,
		dto.Content.Title, dto.Content.Text, dto.Content.URL, dto.IsActive, dto.BannerID,
	)

	_, err = s.client.Exec(ctx, query)
	if err != nil {
		slog.Error("error updating banners",
			"error", err,
		)
		return errors.NewDomainError(errors.ErrDB, "")
	}

	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("error commiting transaction",
			"error", err,
		)
		return errors.NewDomainError(errors.ErrDB, "")
	}

	return nil

}

func isUnique(ctx context.Context, tx pgx.Tx, tagsID []int64, featureID int64) (bool, error) {
	tagsString := fmt.Sprint(tagsID)
	slog.Debug("tags string", "string", tagsString)

	query := fmt.Sprintf(
		`SELECT
			banner_id
		FROM
			banner_tag
		GROUP BY
			"banner_id"
		HAVING
			"tag_id" IN %s AND COUNT(*) = %d`,
		tagsString, len(tagsID),
	)

	rows, err := tx.Query(ctx, query)
	if err != nil {
		slog.Error("error selecting from db",
			"error", err,
		)
		return false, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDs, err := pgx.CollectRows[int64](rows, func(row pgx.CollectableRow) (int64, error) {
		var id int64
		err := row.Scan(&id)
		return id, err
	})

	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return false, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDsString := fmt.Sprint(bannerIDs) // TODO:

	query = fmt.Sprintf(
		`SELECT CASE WHEN EXISTS (
			SELECT *
			FROM banner_feature
			WHERE banner_id IN %s AND feature_id = %d
		)
		THEN FALSE
		ELSE TRUE END;`,
		bannerIDsString, featureID,
	)

	row := tx.QueryRow(ctx, query)

	var unique bool
	err = row.Scan(&unique)

	if err != nil {
		slog.Error("error scanning row",
			"error", err,
		)
		return false, errors.NewDomainError(errors.ErrDB, "")
	}

	return unique, nil
}
