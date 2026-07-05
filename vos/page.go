package vos

type PageQuery struct {
	Page int    `form:"page" json:"page"`
	Size int    `form:"size" json:"size"`
	Q    string `form:"q" json:"q"`
}

func (q *PageQuery) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Size <= 0 {
		q.Size = 20
	}
	if q.Size > 100 {
		q.Size = 100
	}
}

func (q PageQuery) Offset() int {
	return (q.Page - 1) * q.Size
}

type PageResult struct {
	List  any   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}
