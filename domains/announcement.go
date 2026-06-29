package domains

import commonDomains "github.com/wfu-work/nav-common-go-lib/domains"

type Announcement struct {
	commonDomains.BaseDataEntity
	Title     string `json:"title" gorm:"column:title;size:160;index;comment:标题"`
	Content   string `json:"content" gorm:"column:content;type:text;comment:内容"`
	Level     string `json:"level" gorm:"column:level;size:30;default:info;index;comment:级别 info/warning/error"`
	Status    int    `json:"status" gorm:"column:status;default:1;index;comment:状态"`
	Popup     bool   `json:"popup" gorm:"column:popup;default:false;comment:是否弹窗"`
	Priority  int    `json:"priority" gorm:"column:priority;default:0;index;comment:优先级"`
	StartTime int64  `json:"startTime" gorm:"column:start_time;default:0;index;comment:生效时间秒"`
	EndTime   int64  `json:"endTime" gorm:"column:end_time;default:0;index;comment:失效时间秒"`
	Remark    string `json:"remark" gorm:"column:remark;size:255;comment:备注"`
}

func (Announcement) TableName() string {
	return "nav_api_announcements"
}
