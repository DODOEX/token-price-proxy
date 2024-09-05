package schema

import (
	"time"

	"gorm.io/gorm"
)

type RequestLog struct {
	IPAddress     string `gorm:"size:45"`
	Endpoint      string `gorm:"size:255"`
	RequestParams string `gorm:"type:text"`
	Response      string `gorm:"type:text"`
	ExecutionTime int64
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"deleted_at"`
}
