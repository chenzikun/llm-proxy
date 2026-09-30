package model

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/env"
	"github.com/zicorn/llm-proxy/pkg/common/helper"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/pkg/common/random"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB
var LOG_DB *gorm.DB

func CreateRootAccountIfNeed() error {
	var user User
	//if user.Status != util.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		username := config.InitialRootUsername
		password := config.InitialRootPassword
		logger.SysLog(fmt.Sprintf("no user exists, creating a root user for you: username is %s, password is %s", username, password))
		if password == "123456" {
			logger.SysLog("WARNING: the default root password is in use, change it after the first login or set INITIAL_ROOT_PASSWORD")
		}
		hashedPassword, err := common.Password2Hash(password)
		if err != nil {
			return err
		}
		accessToken := random.GetUUID()
		if config.InitialRootAccessToken != "" {
			accessToken = config.InitialRootAccessToken
		}
		rootUser := User{
			Username:    username,
			Password:    hashedPassword,
			Role:        RoleRootUser,
			Status:      UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: accessToken,
			Quota:       500000000000000,
		}
		DB.Create(&rootUser)
		if config.InitialRootToken != "" {
			logger.SysLog("creating initial root token as requested")
			token := Token{
				Id:             1,
				UserId:         rootUser.Id,
				Key:            config.InitialRootToken,
				Status:         TokenStatusEnabled,
				Name:           "Initial Root Token",
				CreatedTime:    helper.GetTimestamp(),
				AccessedTime:   helper.GetTimestamp(),
				ExpiredTime:    -1,
				RemainQuota:    500000000000000,
				UnlimitedQuota: true,
			}
			DB.Create(&token)
		}
	}
	return nil
}

// 数据库类型枚举，取 config.DBType
const (
	DBTypeMySQL    = "mysql"
	DBTypePostgres = "postgres"
	DBTypeSQLite   = "sqlite"
)

func chooseDB(envName string) (*gorm.DB, error) {
	// 直接给了连接串就以它为准，按前缀判断类型（老部署走这条）
	if dsn := os.Getenv(envName); dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") {
			return openPostgreSQL(dsn)
		}
		return openMySQL(dsn)
	}

	switch strings.ToLower(config.DBType) {
	case DBTypeMySQL:
		if dsn := mysqlDSNFromEnv(); dsn != "" {
			return openMySQL(dsn)
		}
	case DBTypePostgres:
		if dsn := postgresDSNFromEnv(); dsn != "" {
			return openPostgreSQL(dsn)
		}
	case DBTypeSQLite:
		return openSQLite()
	case "":
		if os.Getenv("MYSQL_MASTER_SERVER") != "" {
			return createMySQLClient()
		}
	default:
		logger.SysLogf("unknown DB_TYPE %q, using SQLite as database", config.DBType)
	}

	// DB_TYPE 没设、或者该类数据库的连接参数不全：回落 SQLite
	return openSQLite()
}

// mysqlDSNFromEnv 按 MYSQL_* 参数拼 MySQL 连接串，参数不全时返回空串
func mysqlDSNFromEnv() string {
	host := os.Getenv("MYSQL_HOST")
	if host == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		env.String("MYSQL_USERNAME", "llm_proxy"),
		os.Getenv("MYSQL_PASSWORD"),
		host,
		env.String("MYSQL_PORT", "3306"),
		env.String("MYSQL_DB", "llm_proxy"))
}

// postgresDSNFromEnv 按 PG_* 参数拼 PostgreSQL 连接串，参数不全时返回空串
func postgresDSNFromEnv() string {
	host := os.Getenv("PG_HOST")
	if host == "" {
		return ""
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		env.String("PG_USER", "llm_proxy"),
		os.Getenv("PG_PASSWORD"),
		host,
		env.String("PG_PORT", "5432"),
		env.String("PG_DB", "llm_proxy"))
}

func openPostgreSQL(dsn string) (*gorm.DB, error) {
	logger.SysLog("using PostgreSQL as database")
	common.UsingPostgreSQL = true
	return gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func openMySQL(dsn string) (*gorm.DB, error) {
	logger.SysLog("using MySQL as database")
	common.UsingMySQL = true
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func createMySQLClient() (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("MYSQL_USERNAME"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_MASTER_SERVER"), "llm_proxy")
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func openSQLite() (*gorm.DB, error) {
	logger.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	dsn := fmt.Sprintf("%s?_busy_timeout=%d", common.SQLitePath, common.SQLiteBusyTimeout)
	return gorm.Open(sqlite.Open(dsn), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func init() {
	var err error
	DB, err = chooseDB("SQL_DSN")
	if err != nil {
		logger.FatalLog("failed to initialize database: " + err.Error())
		return
	}

	sqlDB := setDBConns(DB)

	if !config.IsMasterNode {
		return
	}

	if common.UsingMySQL {
		_, _ = sqlDB.Exec("DROP INDEX idx_channels_key ON channels;") // TODO: delete this line when most users have upgraded
	}

	logger.SysLog("database migration started")
	if err = migrateDB(); err != nil {
		logger.FatalLog("failed to migrate database: " + err.Error())
		return
	}
	logger.SysLog("database migrated")
	InitLogDB()
}

func migrateDB() error {
	var err error
	if err = DB.AutoMigrate(&Channel{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&Token{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&User{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&Option{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&Redemption{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&Ability{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&Channel{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&File{}); err != nil {
		return err
	}
	if err = DB.AutoMigrate(&ModelMeta{}); err != nil {
		return err
	}
	return nil
}

func InitLogDB() {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		return
	}

	logger.SysLog("using secondary database for table logs")
	var err error
	LOG_DB, err = chooseDB("LOG_SQL_DSN")
	if err != nil {
		logger.FatalLog("failed to initialize secondary database: " + err.Error())
		return
	}

	setDBConns(LOG_DB)

	if !config.IsMasterNode {
		return
	}

	logger.SysLog("secondary database migration started")
	err = migrateLOGDB()
	if err != nil {
		logger.FatalLog("failed to migrate secondary database: " + err.Error())
		return
	}
	logger.SysLog("secondary database migrated")
}

// InitModelMetaFromMap 从预定义的映射初始化模型元数据
func InitModelMetaFromMap() {
	logger.SysLog("初始化模型元数据")
	for _, modelMeta := range ModelMetaMap {
		if modelMeta.Model == "" {
			continue
		}
		modelMetaCopy := modelMeta // 创建副本以避免引用问题
		if err := CreateOrUpdateModelMeta(&modelMetaCopy); err != nil {
			logger.SysLog("创建或更新模型元数据失败: " + err.Error())
			// fatal error
			os.Exit(1)
		}
	}
	logger.SysLog("模型元数据初始化完成")
}

func migrateLOGDB() error {
	var err error
	if err = LOG_DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	return nil
}

func setDBConns(db *gorm.DB) *sql.DB {
	if config.DebugSQLEnabled {
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.FatalLog("failed to connect database: " + err.Error())
		return nil
	}

	sqlDB.SetMaxIdleConns(env.Int("SQL_MAX_IDLE_CONNS", 100))
	sqlDB.SetMaxOpenConns(env.Int("SQL_MAX_OPEN_CONNS", 1000))
	sqlDB.SetConnMaxLifetime(time.Second * time.Duration(env.Int("SQL_MAX_LIFETIME", 60)))
	return sqlDB
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}
