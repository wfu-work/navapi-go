package services

import (
	"errors"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
	"navapi-go/domains"
	"navapi-go/dto"
)

type TaskService struct {
	commonServices.CrudService[domains.Task]
}

var TaskServiceApp = new(TaskService)

func (s *TaskService) WithDB(db *gorm.DB) *TaskService {
	cloned := *s
	cloned.CrudService = *s.CrudService.WithDB(db)
	return &cloned
}

func (s *TaskService) Create(task *domains.Task) error {
	if task.TaskID == "" {
		id, err := randomHex(12)
		if err != nil {
			return err
		}
		task.TaskID = "task_" + id
	}
	if task.Status == "" {
		task.Status = "pending"
	}
	if task.Group == "" {
		task.Group = "default"
	}
	return createWithCrud(&s.CrudService, task)
}

func (s *TaskService) Update(task *domains.Task, userGuid string) error {
	if task.TaskID == "" {
		return errors.New("task id is required")
	}
	db := s.DB().Model(&domains.Task{}).Where("task_id = ?", task.TaskID)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	updates := map[string]any{
		"platform":     task.Platform,
		"group_name":   task.Group,
		"channel_guid": task.ProviderGuid,
		"model_name":   task.ModelName,
		"quota":        task.Quota,
		"action":       task.Action,
		"status":       task.Status,
		"fail_reason":  task.FailReason,
		"progress":     task.Progress,
		"data":         task.Data,
		"private_data": task.PrivateData,
	}
	return db.Updates(updates).Error
}

func (s *TaskService) Delete(taskID string, userGuid string) error {
	if taskID == "" {
		return errors.New("task id is required")
	}
	db := s.DB().Where("task_id = ?", taskID)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	return db.Delete(&domains.Task{}).Error
}

func (s *TaskService) List(userGuid string, query dto.PageQuery) (dto.PageResult, error) {
	query.Normalize()
	var tasks []domains.Task
	var total int64
	db := s.DB().Model(&domains.Task{})
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if query.Q != "" {
		db = db.Where("task_id LIKE ? OR model_name LIKE ? OR action LIKE ?", "%"+query.Q+"%", "%"+query.Q+"%", "%"+query.Q+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return dto.PageResult{}, err
	}
	if err := db.Order("id desc").Offset(query.Offset()).Limit(query.Size).Find(&tasks).Error; err != nil {
		return dto.PageResult{}, err
	}
	return dto.PageResult{List: tasks, Total: total, Page: query.Page, Size: query.Size}, nil
}

func (s *TaskService) Get(taskID string, userGuid string) (*domains.Task, error) {
	var task domains.Task
	db := s.DB().Where("task_id = ?", taskID)
	if userGuid != "" {
		db = db.Where("user_guid = ?", userGuid)
	}
	if err := db.First(&task).Error; err != nil {
		return nil, err
	}
	task.PrivateData = ""
	return &task, nil
}
