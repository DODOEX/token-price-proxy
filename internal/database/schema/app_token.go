package schema

type AppToken struct {
	Name  string  `gorm:"varchar(255); notNull;" json:"name"`
	Token string  `gorm:"varchar(255); notNull;" json:"token"`
	Rate  float64 `gorm:"type:real; notNull;" json:"rate"` // 每秒释放量
	Base
}
