package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const defaultLockName = "go_mall_schema_migrations"

const createMigrationTableSQL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT NOT NULL,
    name VARCHAR(160) NOT NULL,
    checksum CHAR(64) NOT NULL,
    dirty BOOLEAN NOT NULL DEFAULT TRUE,
    applied_at DATETIME(3) NOT NULL,
    PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

type Status struct {
	Migration Migration
	Applied   bool
	AppliedAt time.Time
}

type Runner struct {
	db          *sql.DB
	migrations  []Migration
	lockName    string
	lockTimeout time.Duration
}

func NewRunner(db *sql.DB, migrations []Migration) (*Runner, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接不能为空")
	}
	if err := ValidateMigrations(migrations); err != nil {
		return nil, err
	}
	return &Runner{db: db, migrations: append([]Migration(nil), migrations...), lockName: defaultLockName, lockTimeout: 30 * time.Second}, nil
}

func (r *Runner) Verify(ctx context.Context) error {
	return r.withLock(ctx, func(conn *sql.Conn) error {
		if err := ensureMigrationTable(ctx, conn); err != nil {
			return err
		}
		applied, err := readApplied(ctx, conn)
		if err != nil {
			return err
		}
		return ValidateHistory(r.migrations, applied)
	})
}

func (r *Runner) Status(ctx context.Context) ([]Status, error) {
	var result []Status
	err := r.withLock(ctx, func(conn *sql.Conn) error {
		if err := ensureMigrationTable(ctx, conn); err != nil {
			return err
		}
		applied, err := readApplied(ctx, conn)
		if err != nil {
			return err
		}
		if err := ValidateHistory(r.migrations, applied); err != nil {
			return err
		}
		result = make([]Status, 0, len(r.migrations))
		for index, item := range r.migrations {
			status := Status{Migration: item}
			if index < len(applied) {
				status.Applied = true
				status.AppliedAt = applied[index].AppliedAt
			}
			result = append(result, status)
		}
		return nil
	})
	return result, err
}

func (r *Runner) Up(ctx context.Context) ([]Migration, error) {
	var completed []Migration
	err := r.withLock(ctx, func(conn *sql.Conn) error {
		if err := ensureMigrationTable(ctx, conn); err != nil {
			return err
		}
		applied, err := readApplied(ctx, conn)
		if err != nil {
			return err
		}
		pending, err := Pending(r.migrations, applied)
		if err != nil {
			return err
		}
		for _, item := range pending {
			if err := markDirty(ctx, conn, item); err != nil {
				return err
			}
			if _, err := conn.ExecContext(ctx, item.UpSQL); err != nil {
				return fmt.Errorf("执行 migration %06d_%s 失败（已标记 dirty）: %w", item.Version, item.Name, err)
			}
			if _, err := conn.ExecContext(ctx, `UPDATE schema_migrations SET dirty = FALSE, applied_at = UTC_TIMESTAMP(3) WHERE version = ?`, item.Version); err != nil {
				return fmt.Errorf("完成 migration %06d_%s 后更新状态失败（已标记 dirty）: %w", item.Version, item.Name, err)
			}
			completed = append(completed, item)
		}
		return nil
	})
	return completed, err
}

func (r *Runner) Down(ctx context.Context, steps int) ([]Migration, error) {
	if steps <= 0 {
		return nil, fmt.Errorf("回滚步数必须大于 0")
	}
	var completed []Migration
	err := r.withLock(ctx, func(conn *sql.Conn) error {
		if err := ensureMigrationTable(ctx, conn); err != nil {
			return err
		}
		applied, err := readApplied(ctx, conn)
		if err != nil {
			return err
		}
		if err := ValidateHistory(r.migrations, applied); err != nil {
			return err
		}
		if steps > len(applied) {
			return fmt.Errorf("只能回滚 %d 个已执行 migration，收到 %d", len(applied), steps)
		}
		for index := len(applied) - 1; index >= len(applied)-steps; index-- {
			item := r.migrations[index]
			if _, err := conn.ExecContext(ctx, `UPDATE schema_migrations SET dirty = TRUE WHERE version = ?`, item.Version); err != nil {
				return fmt.Errorf("标记 migration %06d_%s 回滚中失败: %w", item.Version, item.Name, err)
			}
			if _, err := conn.ExecContext(ctx, item.DownSQL); err != nil {
				return fmt.Errorf("回滚 migration %06d_%s 失败（已标记 dirty）: %w", item.Version, item.Name, err)
			}
			if _, err := conn.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = ?`, item.Version); err != nil {
				return fmt.Errorf("回滚 migration %06d_%s 后删除记录失败（已标记 dirty）: %w", item.Version, item.Name, err)
			}
			completed = append(completed, item)
		}
		return nil
	})
	return completed, err
}

func (r *Runner) withLock(ctx context.Context, callback func(*sql.Conn) error) error {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取 migration 专用连接: %w", err)
	}
	defer conn.Close()

	seconds := int(r.lockTimeout.Seconds())
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, r.lockName, seconds).Scan(&acquired); err != nil {
		return fmt.Errorf("获取 migration 锁: %w", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		return fmt.Errorf("%s 秒内未获得 migration 锁", r.lockTimeout)
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var released sql.NullInt64
		_ = conn.QueryRowContext(releaseCtx, `SELECT RELEASE_LOCK(?)`, r.lockName).Scan(&released)
	}()

	return callback(conn)
}

func ensureMigrationTable(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, createMigrationTableSQL); err != nil {
		return fmt.Errorf("创建 schema_migrations: %w", err)
	}
	return nil
}

func readApplied(ctx context.Context, conn *sql.Conn) ([]AppliedMigration, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version, name, checksum, dirty, applied_at FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("读取 schema_migrations: %w", err)
	}
	defer rows.Close()

	result := make([]AppliedMigration, 0)
	for rows.Next() {
		var item AppliedMigration
		if err := rows.Scan(&item.Version, &item.Name, &item.Checksum, &item.Dirty, &item.AppliedAt); err != nil {
			return nil, fmt.Errorf("解析 schema_migrations: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 schema_migrations: %w", err)
	}
	return result, nil
}

func markDirty(ctx context.Context, conn *sql.Conn, item Migration) error {
	_, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, checksum, dirty, applied_at) VALUES (?, ?, ?, TRUE, UTC_TIMESTAMP(3))`, item.Version, item.Name, item.Checksum)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return fmt.Errorf("登记 migration %06d_%s: %w", item.Version, item.Name, err)
	}
	return nil
}
