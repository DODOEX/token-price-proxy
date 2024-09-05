package schema

import (
	"time"

	"gorm.io/gorm"
)

type SlackNotifications struct {
	Source    string         `gorm:"not null"`
	CoinID    string         `gorm:"not null"`
	DayDate   string         `gorm:"not null"`
	Date      int64          `gorm:"not null"`
	Counter   int            `gorm:"default:1"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at"`
}
