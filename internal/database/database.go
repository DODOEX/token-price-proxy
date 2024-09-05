package database

import (
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// setup database with gorm
type Database struct {
	DB  *gorm.DB
	Log zerolog.Logger
	Cfg *koanf.Koanf
}

type Seeder interface {
	Seed(*gorm.DB) error
	Count(*gorm.DB) (int, error)
}

func NewDatabase(cfg *koanf.Koanf, log zerolog.Logger) *Database {
	db := &Database{
		Cfg: cfg,
		Log: log,
	}

	return db
}

// connect database
func (_db *Database) ConnectDatabase() {
	if (_db.DB != nil) && (_db.DB.Migrator().HasTable(&schema.Coins{})) {
		_db.Log.Info().Msg("The database is already connected!")
		return
	}

	conn, err := gorm.Open(postgres.Open(_db.Cfg.String("db.postgres.dsn")), &gorm.Config{
		SkipDefaultTransaction:                   true,
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: _db.Cfg.Bool("db.gorm.disable-foreign-key-constraint-when-migrating"),
	})
	// _db.Log.Info().Msg("Connected database dns " + _db.Cfg.String("db.postgres.dsn"))
	if err != nil {
		_db.Log.Error().Err(err).Msg("An unknown error occurred when to connect the database!")
	} else {
		_db.Log.Info().Msg("Connected the database succesfully!")
	}

	_db.DB = conn
}

// shutdown database
func (_db *Database) ShutdownDatabase() {
	sqlDB, err := _db.DB.DB()
	if err != nil {
		_db.Log.Error().Err(err).Msg("An unknown error occurred when to shutdown the database!")
	} else {
		_db.Log.Info().Msg("Shutdown the database succesfully!")
	}
	sqlDB.Close()
}

// list of models for migration
func Models() []interface{} {
	return []interface{}{
		schema.AppToken{},
		schema.Coins{},
		schema.CoinHistoricalPrice{},
	}
}

// migrate models
func (_db *Database) MigrateModels() {
	if err := _db.DB.AutoMigrate(
		Models()...,
	); err != nil {
		_db.Log.Error().Err(err).Msg("An unknown error occurred when to migrate the database!")
	}
}

// list of models for migration
func Seeders() []Seeder {
	return []Seeder{}
}

// seed data
func (_db *Database) SeedModels() {
	seeders := Seeders()
	for _, seed := range seeders {
		count, err := seed.Count(_db.DB)
		if err != nil {
			_db.Log.Error().Err(err).Msg("An unknown error occurred when to seed the database!")
		}

		if count == 0 {
			if err := seed.Seed(_db.DB); err != nil {
				_db.Log.Error().Err(err).Msg("An unknown error occurred when to seed the database!")
			}

			_db.Log.Info().Msg("Seeded the database succesfully!")
		} else {
			_db.Log.Info().Msg("Database is already seeded!")
		}
	}

	_db.Log.Info().Msg("Seeded the database succesfully!")
}
