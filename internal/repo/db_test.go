package model

import (
	"path/filepath"
	"testing"

	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

func TestMySQLDSNFromEnv(t *testing.T) {
	for _, key := range []string{"MYSQL_HOST", "MYSQL_PORT", "MYSQL_USERNAME", "MYSQL_PASSWORD", "MYSQL_DB"} {
		t.Setenv(key, "")
	}

	if dsn := mysqlDSNFromEnv(); dsn != "" {
		t.Errorf("缺少 MYSQL_HOST 时 = %q，期望空串", dsn)
	}

	t.Setenv("MYSQL_HOST", "10.0.0.5")
	want := "llm_proxy:@tcp(10.0.0.5:3306)/llm_proxy?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn := mysqlDSNFromEnv(); dsn != want {
		t.Errorf("默认参数 = %q，期望 %q", dsn, want)
	}

	t.Setenv("MYSQL_PORT", "3307")
	t.Setenv("MYSQL_USERNAME", "u")
	t.Setenv("MYSQL_PASSWORD", "p")
	t.Setenv("MYSQL_DB", "d")
	want = "u:p@tcp(10.0.0.5:3307)/d?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn := mysqlDSNFromEnv(); dsn != want {
		t.Errorf("自定义参数 = %q，期望 %q", dsn, want)
	}
}

func TestPostgresDSNFromEnv(t *testing.T) {
	for _, key := range []string{"PG_HOST", "PG_PORT", "PG_USER", "PG_PASSWORD", "PG_DB"} {
		t.Setenv(key, "")
	}

	if dsn := postgresDSNFromEnv(); dsn != "" {
		t.Errorf("缺少 PG_HOST 时 = %q，期望空串", dsn)
	}

	t.Setenv("PG_HOST", "10.0.0.6")
	want := "postgres://llm_proxy:@10.0.0.6:5432/llm_proxy?sslmode=disable"
	if dsn := postgresDSNFromEnv(); dsn != want {
		t.Errorf("默认参数 = %q，期望 %q", dsn, want)
	}

	t.Setenv("PG_PORT", "5433")
	t.Setenv("PG_USER", "u")
	t.Setenv("PG_PASSWORD", "p")
	t.Setenv("PG_DB", "d")
	want = "postgres://u:p@10.0.0.6:5433/d?sslmode=disable"
	if dsn := postgresDSNFromEnv(); dsn != want {
		t.Errorf("自定义参数 = %q，期望 %q", dsn, want)
	}
}

// chooseDB 在 DB_TYPE 未设或参数不全时必须回落 SQLite，而不是报错退出
func TestChooseDBFallbackToSQLite(t *testing.T) {
	origPath := common.SQLitePath
	origUsingSQLite := common.UsingSQLite
	origUsingMySQL := common.UsingMySQL
	origUsingPostgreSQL := common.UsingPostgreSQL
	origDBType := config.DBType
	t.Cleanup(func() {
		common.SQLitePath = origPath
		common.UsingSQLite = origUsingSQLite
		common.UsingMySQL = origUsingMySQL
		common.UsingPostgreSQL = origUsingPostgreSQL
		config.DBType = origDBType
	})
	common.SQLitePath = filepath.Join(t.TempDir(), "test.db")

	t.Setenv("SQL_DSN", "")
	t.Setenv("MYSQL_MASTER_SERVER", "")
	t.Setenv("MYSQL_HOST", "")
	t.Setenv("PG_HOST", "")

	cases := []struct {
		name   string
		dbType string
	}{
		{"DB_TYPE 未设置", ""},
		{"DB_TYPE=mysql 但缺 MYSQL_HOST", DBTypeMySQL},
		{"DB_TYPE=postgres 但缺 PG_HOST", DBTypePostgres},
		{"DB_TYPE 取值无法识别", "oracle"},
	}
	for _, tc := range cases {
		config.DBType = tc.dbType
		common.UsingSQLite = false
		common.UsingMySQL = false
		common.UsingPostgreSQL = false

		db, err := chooseDB("SQL_DSN")
		if err != nil {
			t.Fatalf("%s: chooseDB 返回错误: %v", tc.name, err)
		}
		if db == nil {
			t.Fatalf("%s: chooseDB 返回了 nil 连接", tc.name)
		}
		if !common.UsingSQLite {
			t.Errorf("%s: 期望回落 SQLite，实际 UsingSQLite=false", tc.name)
		}
	}
}
