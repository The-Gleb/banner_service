package db

import (
	"context"
	"embed"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/domain/service"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/The-Gleb/banner_service/pkg/client/postgresql"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ service.BannerStorage = new(bannerStorage)

type bannerStorage struct {
	client postgresql.Client
}

func NewBannerStorage(client postgresql.Client) *bannerStorage {
	return &bannerStorage{client: client}
}

//go:embed migration/*.sql
var migrationsDir embed.FS

func RunMigrations(dsn string) error {

	d, err := iofs.New(migrationsDir, "migration")
	if err != nil {
		slog.Error(err.Error())
		return fmt.Errorf("failed to return an iofs driver: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	if err != nil {
		slog.Error(err.Error())
		return fmt.Errorf("failed to get a new migrate instance: %w", err)
	}
	if err := m.Up(); err != nil {
		slog.Error(err.Error())
		if !stdErrors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to apply migrations to the DB: %w", err)
		}
	}
	return nil
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
		WHERE tag_id = %d;`,
		dto.TagID,
	)

	rows, err := tx.Query(ctx, query)
	if err != nil {
		slog.Error("error selecting from banner_tag",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	strBannerIDs, err := pgx.CollectRows[string](rows, func(row pgx.CollectableRow) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})

	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`SELECT banner_id
		FROM banner_feature
		WHERE banner_id IN (%s) AND feature_id = %d;`,
		strings.Join(strBannerIDs, ", "), dto.FeatureID,
	)

	row := tx.QueryRow(ctx, query)

	var bannerID int64
	err = row.Scan(&bannerID)
	if err != nil {
		slog.Error("error scanning row",
			"error", err,
		)
		if stdErrors.Is(err, pgx.ErrNoRows) {
			return entity.UpdateCacheDTO{}, errors.NewDomainError(errors.ErrNoDataFound, "")
		}

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

func (s *bannerStorage) GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.Banner, error) {

	tx, err := s.client.Begin(ctx)
	if err != nil {
		slog.Error("error beginnig transaction",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}
	defer tx.Rollback(ctx)

	// tagID, tagOK := dto.Filters["tag"]
	// featureID, featureOK := dto.Filters["feature"]

	query := `SELECT
    bt.banner_id,
    feature_id,
    ARRAY_AGG(tag_id)
	FROM
		banner_tag bt
		JOIN banner_feature bf
		ON bt.banner_id = bf.banner_id
	WHERE
		bt.banner_id IN (
        SELECT
            bt.banner_id
        FROM
            banner_tag bt
            JOIN banner_feature bf ON bt.banner_id = bf.banner_id
        WHERE
	`

	for k, v := range dto.Filters {
		query = fmt.Sprintf(
			`%s
			%s_id = %d
			AND`,
			query, k, v,
		)
	}

	query = strings.TrimSuffix(query, "AND")

	// if tagOK {
	// 	query = fmt.Sprintf(
	// 		`%s
	// 		WHERE tag_id = %d
	// 			`,
	// 		query, tagID,
	// 	)
	// }

	// query += "GROUP BY bt.banner_id, feature_id"

	// if featureOK {
	// 	query = fmt.Sprintf(
	// 		`%s
	// 		HAVING feature_id = %d
	// 		`,
	// 		query, featureID,
	// 	)
	// }

	query = fmt.Sprintf(
		`%s
		LIMIT %d
		OFFSET %d)
		GROUP BY
			bt.banner_id,
			feature_id;`,
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

	bannerIndexes := make(map[int64]int)
	bannerSlice := make([]entity.Banner, 0)
	index := 0

	bannerIDs, err := pgx.CollectRows[string](rows, func(row pgx.CollectableRow) (string, error) {
		var banner entity.Banner
		var stringTags pgtype.FlatArray[int64]
		err := rows.Scan(&banner.BannerID, &banner.FeatureID, &stringTags)
		if err != nil {
			slog.Error("error scanning row",
				"error", err,
			)
			return "", errors.NewDomainError(errors.ErrDB, "")
		}

		banner.TagIDs = stringTags

		bannerSlice = append(bannerSlice, banner)
		bannerIndexes[banner.BannerID] = index
		index++

		return strconv.FormatInt(banner.BannerID, 10), nil
	})
	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`SELECT id, title, text, url
		FROM banners
		WHERE id IN (%s);`,
		strings.Join(bannerIDs, ", "),
	)

	rows, err = tx.Query(ctx, query)
	if err != nil {
		slog.Error("error selecting banners content from db",
			"error", err,
		)
		return nil, errors.NewDomainError(errors.ErrDB, "")
	}

	for rows.Next() {
		var (
			id    int64
			title string
			text  string
			url   string
		)
		err := rows.Scan(&id, &title, &text, &url)
		if err != nil {
			slog.Error("error scanning rows",
				"error", err,
			)
			return nil, errors.NewDomainError(errors.ErrDB, "")
		}

		bannerSlice[bannerIndexes[id]].Content.Title = title
		bannerSlice[bannerIndexes[id]].Content.Text = text
		bannerSlice[bannerIndexes[id]].Content.URL = url

	}

	if err := rows.Err(); err != nil {
		slog.Error("error scanning rows",
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

	return bannerSlice, nil
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
		slog.Debug("update params violate unique constraint")
		return errors.NewDomainError(errors.ErrAlreadyExists, "banner with these tags and feature already exists")
	}

	query := fmt.Sprintf(
		`DELETE FROM banner_tag
		WHERE banner_id = %d;`,
		dto.BannerID,
	)

	c, err := tx.Exec(ctx, query)

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

	slog.Debug("tags", "slice", dto.TagIDs)

	tagsString := ""
	for _, tagID := range dto.TagIDs {
		tagsString += fmt.Sprintf("(%d, %d),", dto.BannerID, tagID)
	}
	tagsString = strings.TrimSuffix(tagsString, ",")

	query = fmt.Sprintf(
		`INSERT INTO
				banner_tag ("banner_id", "tag_id")
			VALUES
				%s;`,
		tagsString,
	)

	slog.Info(query)

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

	// query = fmt.Sprintf(
	// 	`UPDATE banner_feature
	// 		SET feature_id = %d
	// 		WHERE banner_id = %d;`,
	// 	dto.FeatureID, dto.BannerID,
	// )

	// _, err = s.client.Exec(ctx, query)
	// if err != nil {
	// 	slog.Error("error updating banner_feature",
	// 		"error", err,
	// 	)
	// 	var pgErr *pgconn.PgError
	// 	if stdErrors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
	// 		return errors.NewDomainError(errors.ErrTagNotFound, "")
	// 	}
	// 	return errors.NewDomainError(errors.ErrDB, "")
	// }

	// query = fmt.Sprintf(
	// 	`UPDATE banners
	// 		SET title = '%s'
	// 		WHERE id = %d;`,
	// 	dto.Content.Title, dto.BannerID,
	// )

	// _, err = s.client.Exec(ctx, query)
	// if err != nil {
	// 	slog.Error("error updating banners",
	// 		"error", err,
	// 	)
	// 	return errors.NewDomainError(errors.ErrDB, "")
	// }

	// query = fmt.Sprintf(
	// 	`UPDATE banners
	// 		SET text = '%s'
	// 		WHERE id = %d;`,
	// 	dto.Content.Text, dto.BannerID,
	// )

	// _, err = s.client.Exec(ctx, query)
	// if err != nil {
	// 	slog.Error("error updating banners",
	// 		"error", err,
	// 	)
	// 	return errors.NewDomainError(errors.ErrDB, "")
	// }

	// query = fmt.Sprintf(
	// 	`UPDATE banners
	// 		SET url = '%s'
	// 		WHERE id = %d;`,
	// 	dto.Content.URL, dto.BannerID,
	// )

	// _, err = s.client.Exec(ctx, query)
	// if err != nil {
	// 	slog.Error("error updating banners",
	// 		"error", err,
	// 	)
	// 	return errors.NewDomainError(errors.ErrDB, "")
	// }

	// query = fmt.Sprintf(
	// 	`UPDATE banners
	// 	SET is_active = %t, updated_at = NOW()
	// 	WHERE id = %d;`,
	// 	dto.IsActive, dto.BannerID,
	// )

	// _, err = s.client.Exec(ctx, query)
	// if err != nil {
	// 	slog.Error("error updating banners",
	// 		"error", err,
	// 	)
	// 	return errors.NewDomainError(errors.ErrDB, "")
	// }

	tx.Rollback(ctx)
	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("error commiting transaction",
			"error", err,
		)
		return errors.NewDomainError(errors.ErrDB, "")
	}

	return nil

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
			('%s', '%s', '%s', %t, NOW())
		RETURNING id;`,
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

	tagsString := ""
	for _, tagID := range dto.TagIDs {
		tagsString += fmt.Sprintf("(%d, %d),", bannerID, tagID)
	}
	tagsString = strings.TrimSuffix(tagsString, ",")

	query = fmt.Sprintf(
		`INSERT INTO
			banner_tag ("banner_id", "tag_id")
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
			banner_feature ("banner_id", "feature_id")
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

func isUnique(ctx context.Context, tx pgx.Tx, tagsID []int64, featureID int64) (bool, error) {
	tagsString := strings.Trim(strings.ReplaceAll(fmt.Sprint(tagsID), " ", ", "), "[]")
	slog.Debug("tags string", "string", tagsString)

	query := fmt.Sprintf(
		`SELECT banner_id
		FROM banner_tag
			WHERE banner_id NOT IN(
				SELECT banner_id FROM banner_tag bt
				WHERE bt.tag_id NOT IN(%s)
				GROUP BY banner_id
			)
		GROUP BY banner_id
		HAVING COUNT(tag_id) = %d;`,
		tagsString, len(tagsID),
	)

	rows, err := tx.Query(ctx, query)
	if err != nil {
		slog.Error("error selecting from db",
			"error", err,
		)
		return false, errors.NewDomainError(errors.ErrDB, "")
	}

	bannerIDs, err := pgx.CollectRows[string](rows, func(row pgx.CollectableRow) (string, error) {
		var id string
		err := row.Scan(&id)
		return id, err
	})

	if len(bannerIDs) == 0 {
		return true, nil
	}

	if err != nil {
		slog.Error("error collecting rows",
			"error", err,
		)
		return false, errors.NewDomainError(errors.ErrDB, "")
	}

	query = fmt.Sprintf(
		`SELECT CASE WHEN EXISTS (
			SELECT *
			FROM banner_feature
			WHERE banner_id IN (%s) AND feature_id = %d
		)
		THEN FALSE
		ELSE TRUE END;`,
		strings.Join(bannerIDs, ","), featureID,
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
