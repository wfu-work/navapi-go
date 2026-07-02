package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	commonServices "github.com/wfu-work/nav-common-go-lib/services"
	"gorm.io/gorm"
)

func normalizeGroup(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return "default"
	}
	return group
}

func splitCSV(raw string) []string {
	raw = strings.ReplaceAll(raw, "\n", ",")
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func validateOptionalJSONObject(raw string, field string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fmt.Errorf("%s must be a JSON object: %w", field, err)
	}
	return nil
}

func int64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

type beforeCreateHook interface {
	BeforeCreate(*gorm.DB) error
}

func createWithCrud[T commonServices.HasBaseData](crud *commonServices.CrudService[T], entity *T) error {
	base := (*entity).GetBaseData()
	if base.Guid == "" || base.CreateTime == 0 {
		if hook, ok := any(entity).(beforeCreateHook); ok {
			if err := hook.BeforeCreate(nil); err != nil {
				return err
			}
		}
	}
	if err := crud.Create(*entity); err != nil {
		return err
	}
	return reloadByGuidWithCrud(crud, entity)
}

func reloadByGuidWithCrud[T commonServices.HasBaseData](crud *commonServices.CrudService[T], entity *T) error {
	guid := (*entity).GetBaseData().Guid
	if guid == "" {
		return errors.New("guid is required")
	}
	saved, err := crud.GetByGuid(guid)
	if err != nil {
		return err
	}
	if saved == nil {
		return errors.New("record not found")
	}
	*entity = *saved
	return nil
}

func deleteByIDWithCrud[T commonServices.HasBaseData](crud *commonServices.CrudService[T], id uint, notFoundMessage string) error {
	if id == 0 {
		return errors.New("id is required")
	}
	entity, err := crud.GetById(id)
	if err != nil {
		return err
	}
	if entity == nil {
		return errors.New(notFoundMessage)
	}
	return crud.DeleteByGuid((*entity).GetBaseData().Guid)
}
