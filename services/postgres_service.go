package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"navapi-go/domains"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	postgresUsageLogMigrationKey = "migration.postgres.usage_log_v1"
	postgresMigrationBatchSize   = 500
	postgresMigrationLockID      = int64(1713370001)
)

type PostgresService struct{}

var PostgresServiceApp = new(PostgresService)

type postgresIndex struct {
	name string
	sql  string
}

var postgresIndexes = []postgresIndex{
	{
		name: "idx_nav_api_vendor_active_type_sort",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_vendor_active_type_sort
			ON nav_api_vendor_meta (type, sort DESC, id DESC)
			WHERE deleted_time IS NULL AND enabled = TRUE AND btrim(COALESCE("key", '')) <> ''`,
	},
	{
		name: "idx_nav_api_model_meta_active_sort",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_model_meta_active_sort
			ON nav_api_model_meta (sort DESC, id DESC)
			WHERE deleted_time IS NULL AND enabled = TRUE`,
	},
	{
		name: "idx_nav_api_model_group_provider_order",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_model_group_provider_order
			ON nav_api_model_group_providers (group_guid, sort ASC, id ASC)
			WHERE deleted_time IS NULL`,
	},
	{
		name: "idx_nav_api_usage_logs_source_time_active",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_usage_logs_source_time_active
			ON nav_api_usage_logs (source, create_time DESC, id DESC)
			WHERE deleted_time IS NULL`,
	},
	{
		name: "idx_nav_api_usage_logs_user_source_time_id_active",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_usage_logs_user_source_time_id_active
			ON nav_api_usage_logs (user_guid, source, create_time DESC, id DESC)
			WHERE deleted_time IS NULL`,
	},
	{
		name: "idx_nav_api_usage_logs_source_status_time_active",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_usage_logs_source_status_time_active
			ON nav_api_usage_logs (source, status, create_time DESC, id DESC)
			WHERE deleted_time IS NULL`,
	},
	{
		name: "idx_nav_api_usage_logs_source_model_time_active",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_usage_logs_source_model_time_active
			ON nav_api_usage_logs (source, model_name, create_time DESC)
			WHERE deleted_time IS NULL`,
	},
	{
		name: "idx_nav_api_wallet_records_user_id_active",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_wallet_records_user_id_active
			ON nav_api_user_wallet_records (user_guid, id DESC)
			WHERE deleted_time IS NULL`,
	},
	{
		name: "idx_nav_api_wallet_records_user_time_active",
		sql: `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_nav_api_wallet_records_user_time_active
			ON nav_api_user_wallet_records (user_guid, occurred_at DESC, id DESC)
			WHERE deleted_time IS NULL`,
	},
}

func (s *PostgresService) Ensure(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not initialized")
	}
	if dialect := db.Dialector.Name(); dialect != "postgres" {
		return fmt.Errorf("postgres database is required, got %q", dialect)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", postgresMigrationLockID).Error; err != nil {
			return fmt.Errorf("acquire postgres data migration lock: %w", err)
		}
		if err := migrateLegacyUsageLogs(tx); err != nil {
			return fmt.Errorf("migrate legacy usage logs: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve postgres index connection: %w", err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", postgresMigrationLockID); err != nil {
		return fmt.Errorf("acquire postgres index lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", postgresMigrationLockID)
	}()

	for _, index := range postgresIndexes {
		if err = ensurePostgresIndex(ctx, conn, index); err != nil {
			return fmt.Errorf("create postgres index %s: %w", index.name, err)
		}
	}
	return nil
}

func ensurePostgresIndex(ctx context.Context, conn *sql.Conn, index postgresIndex) error {
	var valid bool
	err := conn.QueryRowContext(ctx, `
		SELECT pg_index.indisvalid
		FROM pg_index
		JOIN pg_class ON pg_class.oid = pg_index.indexrelid
		JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
		WHERE pg_namespace.nspname = current_schema() AND pg_class.relname = $1
	`, index.name).Scan(&valid)
	if err == nil && valid {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if _, err = conn.ExecContext(ctx, `DROP INDEX CONCURRENTLY IF EXISTS "`+index.name+`"`); err != nil {
			return err
		}
	}
	_, err = conn.ExecContext(ctx, index.sql)
	return err
}

func normalizeLegacyUsageLogSources(db *gorm.DB) error {
	return db.Exec(
		"UPDATE nav_api_usage_logs SET source = ? WHERE source IS NULL OR source = ''",
		domains.UsageLogSourceUser,
	).Error
}

func migrateLegacyUsageLogs(db *gorm.DB) error {
	complete, err := postgresMigrationComplete(db, postgresUsageLogMigrationKey)
	if err != nil || complete {
		return err
	}
	if err = normalizeLegacyUsageLogSources(db); err != nil {
		return err
	}

	type legacyUsageLog struct {
		ID    uint
		Other string
	}
	var cursor uint
	for {
		var rows []legacyUsageLog
		err = db.Model(&domains.UsageLog{}).
			Select("id", "other").
			Where("id > ?", cursor).
			Where("COALESCE(cost, 0) = 0").
			Where("btrim(COALESCE(other, '')) <> ''").
			Where("other LIKE ?", `%"finalCost"%`).
			Order("id ASC").
			Limit(postgresMigrationBatchSize).
			Find(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}

		if err = db.Transaction(func(tx *gorm.DB) error {
			for _, row := range rows {
				cost, ok := legacyFinalCost(row.Other)
				if !ok || cost <= 0 {
					continue
				}
				if err := tx.Model(&domains.UsageLog{}).
					Where("id = ?", row.ID).
					UpdateColumn("cost", cost).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		cursor = rows[len(rows)-1].ID
	}

	return markPostgresMigrationComplete(db, postgresUsageLogMigrationKey)
}

func legacyFinalCost(raw string) (float64, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0, false
	}
	value, ok := payload["finalCost"]
	if !ok {
		return 0, false
	}
	var cost float64
	switch typed := value.(type) {
	case float64:
		cost = typed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		cost = parsed
	default:
		return 0, false
	}
	return cost, cost >= 0
}

func postgresMigrationComplete(db *gorm.DB, key string) (bool, error) {
	var option domains.Option
	err := db.Where("key = ?", key).Take(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return option.Value == "completed", nil
}

func markPostgresMigrationComplete(db *gorm.DB, key string) error {
	option := domains.Option{Key: key, Value: "completed"}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&option).Error
}
