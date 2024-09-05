package schema

type CoinHistoricalPrice struct {
	CoinID    string  `gorm:"type:varchar(255);notNull" json:"coin_id"`           // coin id
	Date      int64   `gorm:"type:bigint;notNull" json:"date"`                    // unix date
	DayDate   string  `gorm:"type:varchar(255);notNull" json:"day_date"`          // day date
	Price     string  `gorm:"type:varchar(255);notNull" json:"price"`             // price
	Source    string  `gorm:"type:varchar(255);notNull;default:''" json:"source"` // data source
	QueryInfo *string `gorm:"type:json" json:"query_info"`                        // query info
	Base
}
