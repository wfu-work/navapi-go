package domains

type QuotaDate struct {
	ID        uint   `json:"id" gorm:"primarykey;autoIncrement"`
	Date      string `json:"date" gorm:"column:date;size:20;index:idx_quota_date_user,unique;comment:日期 yyyy-mm-dd"`
	UserGuid  string `json:"userGuid" gorm:"column:user_guid;size:100;index:idx_quota_date_user,unique;comment:用户 GUID"`
	Quota     int64  `json:"quota" gorm:"column:quota;default:0;comment:当日 Token 用量"`
	Requests  int64  `json:"requests" gorm:"column:requests;default:0;comment:请求数"`
	UpdatedAt int64  `json:"updatedAt" gorm:"column:updated_at;comment:更新时间毫秒"`
}

func (QuotaDate) TableName() string {
	return "nav_api_quota_dates"
}
