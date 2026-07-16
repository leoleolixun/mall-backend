package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var migrationFilePattern = regexp.MustCompile(`^(\d{6})_([a-z0-9_]+)\.(up|down)\.sql$`)

type Migration struct {
	Version  int64
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string
}

type AppliedMigration struct {
	Version   int64
	Name      string
	Checksum  string
	Dirty     bool
	AppliedAt time.Time
}

type migrationPair struct {
	version int64
	name    string
	upSQL   string
	downSQL string
}

func LoadFiles(source fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, fmt.Errorf("读取 migration 文件: %w", err)
	}
	pairs := make(map[int64]*migrationPair)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			if strings.HasSuffix(entry.Name(), ".sql") {
				return nil, fmt.Errorf("migration 文件名不合法: %s", entry.Name())
			}
			continue
		}
		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("migration 版本不合法: %s", entry.Name())
		}
		content, err := fs.ReadFile(source, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("读取 %s: %w", entry.Name(), err)
		}
		sqlText := strings.TrimSpace(string(content))
		if sqlText == "" {
			return nil, fmt.Errorf("migration 文件为空: %s", entry.Name())
		}
		pair, exists := pairs[version]
		if !exists {
			pair = &migrationPair{version: version, name: matches[2]}
			pairs[version] = pair
		}
		if pair.name != matches[2] {
			return nil, fmt.Errorf("migration %06d 的 up/down 名称不一致", version)
		}
		switch matches[3] {
		case "up":
			if pair.upSQL != "" {
				return nil, fmt.Errorf("migration %06d 存在重复 up 文件", version)
			}
			pair.upSQL = sqlText
		case "down":
			if pair.downSQL != "" {
				return nil, fmt.Errorf("migration %06d 存在重复 down 文件", version)
			}
			pair.downSQL = sqlText
		}
	}

	versions := make([]int64, 0, len(pairs))
	for version := range pairs {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	result := make([]Migration, 0, len(versions))
	for _, version := range versions {
		pair := pairs[version]
		if pair.upSQL == "" || pair.downSQL == "" {
			return nil, fmt.Errorf("migration %06d 必须同时提供 up 和 down 文件", version)
		}
		hash := sha256.Sum256([]byte(pair.upSQL + "\x00" + pair.downSQL))
		result = append(result, Migration{
			Version:  pair.version,
			Name:     pair.name,
			UpSQL:    pair.upSQL,
			DownSQL:  pair.downSQL,
			Checksum: hex.EncodeToString(hash[:]),
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("没有找到 migration 文件")
	}
	return result, nil
}

func ValidateHistory(local []Migration, applied []AppliedMigration) error {
	if err := ValidateMigrations(local); err != nil {
		return err
	}
	if len(applied) > len(local) {
		return fmt.Errorf("数据库 migration 数量 %d 超过当前二进制 %d", len(applied), len(local))
	}
	for index, item := range applied {
		if item.Dirty {
			return fmt.Errorf("migration %06d_%s 处于 dirty 状态，必须人工核对 schema 后修复", item.Version, item.Name)
		}
		expected := local[index]
		if item.Version != expected.Version {
			return fmt.Errorf("数据库 migration 历史不是当前版本的有序前缀: 位置 %d 为 %06d，预期 %06d", index, item.Version, expected.Version)
		}
		if item.Name != expected.Name {
			return fmt.Errorf("migration %06d 名称不一致: 数据库=%s 当前=%s", item.Version, item.Name, expected.Name)
		}
		if item.Checksum != expected.Checksum {
			return fmt.Errorf("migration %06d_%s checksum 不一致，已执行 SQL 禁止修改", item.Version, item.Name)
		}
	}
	return nil
}

func ValidateMigrations(items []Migration) error {
	if len(items) == 0 {
		return fmt.Errorf("migration 列表不能为空")
	}
	var previousVersion int64
	for index, item := range items {
		if item.Version <= 0 {
			return fmt.Errorf("第 %d 个 migration 版本必须大于 0", index+1)
		}
		if index > 0 && item.Version <= previousVersion {
			return fmt.Errorf("migration 版本必须严格递增: %06d 位于 %06d 之后", item.Version, previousVersion)
		}
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("migration %06d 名称不能为空", item.Version)
		}
		if strings.TrimSpace(item.UpSQL) == "" || strings.TrimSpace(item.DownSQL) == "" {
			return fmt.Errorf("migration %06d_%s 必须同时提供 up 和 down SQL", item.Version, item.Name)
		}
		checksum, err := hex.DecodeString(item.Checksum)
		if err != nil || len(checksum) != sha256.Size {
			return fmt.Errorf("migration %06d_%s checksum 必须是 64 位 SHA-256", item.Version, item.Name)
		}
		previousVersion = item.Version
	}
	return nil
}

func Pending(local []Migration, applied []AppliedMigration) ([]Migration, error) {
	if err := ValidateHistory(local, applied); err != nil {
		return nil, err
	}
	return append([]Migration(nil), local[len(applied):]...), nil
}
