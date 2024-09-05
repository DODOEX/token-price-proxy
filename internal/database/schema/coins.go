package schema

import "time"

type Coins struct {
	ID                   string     `gorm:"type:varchar(255);primaryKey" json:"id"`                  // chain_id + address
	Address              string     `gorm:"type:varchar(255);notNull" json:"address"`                // token or pool address
	ChainID              string     `gorm:"type:varchar(255);notNull" json:"chain_id"`               // chain id
	Symbol               *string    `gorm:"type:varchar(255);notNull" json:"symbol"`                 // symbol
	Name                 *string    `gorm:"type:varchar(255);notNull" json:"name"`                   // name
	CoingeckoCoinID      *string    `gorm:"type:varchar(255)" json:"coingecko_coin_id"`              // coingecko coin id
	CoingeckoPlatforms   JSONMap    `gorm:"type:json" json:"coingecko_platforms"`                    // coingecko platforms
	GeckoterminalNetwork *string    `gorm:"type:varchar(255)" json:"geckoterminal_network"`          // geckoterminal network
	Extra                *string    `gorm:"type:json" json:"extra"`                                  // json扩展信息
	Decimals             *int       `gorm:"type:int" json:"decimals"`                                // decimals
	TotalSupply          *string    `gorm:"type:varchar(255)" json:"total_supply"`                   // total supply
	Label                string     `gorm:"type:varchar(255);notNull;default:''" json:"label"`       // label
	PoolName             *string    `gorm:"type:varchar(255);default:''" json:"pool_name"`           // pool name
	BaseTokenAddress     *string    `gorm:"type:varchar(255);default:''" json:"base_token_address"`  // base token address
	QuoteTokenAddress    *string    `gorm:"type:varchar(255);default:''" json:"quote_token_address"` // quote token address
	PoolCreatedAt        *time.Time `gorm:"" json:"pool_created_at"`                                 // pool created at
	PoolAttributes       *string    `gorm:"type:json" json:"pool_attributes"`                        // pool attributes
	LastPriceSource      *string    `gorm:"type:varchar(255)" json:"last_price_source"`              // 上次价格查询结果来源
	PriceSource          *string    `gorm:"type:varchar(255)" json:"price_source"`                   // 价格查询来源
	ReturnCoinsId        *string    `gorm:"type:varchar(255)" json:"return_coins_id"`                // 返回的coins ID
	Base
}
