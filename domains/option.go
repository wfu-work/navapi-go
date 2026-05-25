package domains

type Option struct {
	Key   string `json:"key" gorm:"column:key;primaryKey;size:120"`
	Value string `json:"value" gorm:"column:value;type:text"`
}

func (Option) TableName() string {
	return "nav_api_options"
}
