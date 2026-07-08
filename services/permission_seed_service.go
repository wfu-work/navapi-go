package services

import (
	"errors"

	commonDomains "github.com/wfu-work/nav-common-go-lib/domains"
	"github.com/wfu-work/nav-common-go-lib/global"
	commonMiddlewares "github.com/wfu-work/nav-common-go-lib/middlewares"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	roleGuidCompanyAdmin = "2a9ed1e6a5a334461b263e5a80207cc1"
	roleGuidUser         = "3a9ed1e6a5a334461b263e5a80207cc3"
)

type PermissionSeedService struct{}

type apiPermissionSeed struct {
	Guid  string
	Name  string
	Code  string
	Path  string
	Verb  string
	User  bool
	Sort  int
	Group string
}

var PermissionSeedServiceApp = new(PermissionSeedService)

func (s *PermissionSeedService) Ensure() error {
	db := global.NAV_DB
	if db == nil {
		return errors.New("database is not initialized")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		permissions := make([]commonDomains.SysPermission, 0, len(navapiAPIPermissionSeeds))
		for _, seed := range navapiAPIPermissionSeeds {
			permissions = append(permissions, commonDomains.SysPermission{
				BaseDataEntity: commonDomains.BaseDataEntity{Guid: seed.Guid},
				Name:           seed.Name,
				Code:           seed.Code,
				Type:           commonDomains.PermissionTypeAPI,
				Path:           seed.Path,
				Method:         seed.Verb,
				Enabled:        true,
				Sort:           seed.Sort,
				Remark:         seed.Group,
			})
		}
		if len(permissions) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "guid"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"name",
					"code",
					"type",
					"path",
					"method",
					"enabled",
					"sort",
					"remark",
				}),
			}).Create(&permissions).Error; err != nil {
				return err
			}
		}

		userDeniedPermissionGuids := make([]string, 0, len(navapiAPIPermissionSeeds))
		relations := make([]commonDomains.SysRolePermission, 0, len(navapiAPIPermissionSeeds)*2)
		for _, seed := range navapiAPIPermissionSeeds {
			relations = append(relations, commonDomains.SysRolePermission{
				RoleGuid:       roleGuidCompanyAdmin,
				PermissionGuid: seed.Guid,
			})
			if seed.User {
				relations = append(relations, commonDomains.SysRolePermission{
					RoleGuid:       roleGuidUser,
					PermissionGuid: seed.Guid,
				})
			} else {
				userDeniedPermissionGuids = append(userDeniedPermissionGuids, seed.Guid)
			}
		}
		// 同步移除普通用户角色上残留的后台权限，避免历史种子关系在升级后继续生效。
		if len(userDeniedPermissionGuids) > 0 {
			if err := tx.Where("role_guid = ? AND permission_guid IN ?", roleGuidUser, userDeniedPermissionGuids).
				Delete(&commonDomains.SysRolePermission{}).Error; err != nil {
				return err
			}
		}
		if len(relations) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&relations).Error
	}); err != nil {
		return err
	}
	return commonMiddlewares.RefreshCasbin()
}

var navapiAPIPermissionSeeds = []apiPermissionSeed{
	{Guid: "navapi-api-user-token", Name: "当前用户信息", Code: "navapi.user.token", Path: "/user/token", Verb: "GET", User: true, Sort: 10, Group: "基础用户"},
	{Guid: "navapi-api-user-password", Name: "修改当前用户密码", Code: "navapi.user.password.update", Path: "/user/update/password", Verb: "PUT", User: true, Sort: 11, Group: "基础用户"},

	{Guid: "navapi-api-gateway-health", Name: "网关健康检查", Code: "navapi.gateway.health", Path: "/gateway/health", Verb: "GET", Sort: 100, Group: "网关"},

	{Guid: "navapi-api-token-usage", Name: "令牌用量", Code: "navapi.token.usage", Path: "/usage/token", Verb: "GET", Sort: 200, Group: "令牌"},
	{Guid: "navapi-api-token-self-list", Name: "当前用户令牌列表", Code: "navapi.token.self.list", Path: "/token/self/list", Verb: "GET", User: true, Sort: 201, Group: "令牌"},
	{Guid: "navapi-api-token-self-get", Name: "当前用户令牌详情", Code: "navapi.token.self.get", Path: "/token/self/:id", Verb: "GET", User: true, Sort: 202, Group: "令牌"},
	{Guid: "navapi-api-token-self-create", Name: "当前用户创建令牌", Code: "navapi.token.self.create", Path: "/token/self", Verb: "POST", User: true, Sort: 203, Group: "令牌"},
	{Guid: "navapi-api-token-self-update", Name: "当前用户更新令牌", Code: "navapi.token.self.update", Path: "/token/self", Verb: "PUT", User: true, Sort: 204, Group: "令牌"},
	{Guid: "navapi-api-token-self-delete", Name: "当前用户删除令牌", Code: "navapi.token.self.delete", Path: "/token/self/:id", Verb: "DELETE", User: true, Sort: 205, Group: "令牌"},
	{Guid: "navapi-api-token-self-key", Name: "当前用户查看令牌密钥", Code: "navapi.token.self.key", Path: "/token/self/:id/key", Verb: "POST", User: true, Sort: 206, Group: "令牌"},
	{Guid: "navapi-api-token-list", Name: "令牌列表", Code: "navapi.token.list", Path: "/token/list", Verb: "GET", Sort: 220, Group: "令牌"},
	{Guid: "navapi-api-token-get", Name: "令牌详情", Code: "navapi.token.get", Path: "/token/:id", Verb: "GET", Sort: 221, Group: "令牌"},
	{Guid: "navapi-api-token-create", Name: "创建令牌", Code: "navapi.token.create", Path: "/token/", Verb: "POST", Sort: 222, Group: "令牌"},
	{Guid: "navapi-api-token-update", Name: "更新令牌", Code: "navapi.token.update", Path: "/token/", Verb: "PUT", Sort: 223, Group: "令牌"},
	{Guid: "navapi-api-token-delete", Name: "删除令牌", Code: "navapi.token.delete", Path: "/token/:id", Verb: "DELETE", Sort: 224, Group: "令牌"},
	{Guid: "navapi-api-token-key", Name: "查看令牌密钥", Code: "navapi.token.key", Path: "/token/:id/key", Verb: "POST", Sort: 225, Group: "令牌"},

	{Guid: "navapi-api-model-list", Name: "模型列表", Code: "navapi.model.list", Path: "/models/list", Verb: "GET", User: true, Sort: 300, Group: "模型"},
	{Guid: "navapi-api-model-groups", Name: "模型分组列表", Code: "navapi.model.groups", Path: "/models/groups", Verb: "GET", User: true, Sort: 301, Group: "模型"},
	{Guid: "navapi-api-model-group-create", Name: "创建模型分组", Code: "navapi.model.group.create", Path: "/models/groups", Verb: "POST", Sort: 302, Group: "模型"},
	{Guid: "navapi-api-model-group-update", Name: "更新模型分组", Code: "navapi.model.group.update", Path: "/models/groups", Verb: "PUT", Sort: 303, Group: "模型"},
	{Guid: "navapi-api-model-group-delete", Name: "删除模型分组", Code: "navapi.model.group.delete", Path: "/models/groups/:guid", Verb: "DELETE", Sort: 304, Group: "模型"},
	{Guid: "navapi-api-model-create", Name: "创建模型", Code: "navapi.model.create", Path: "/models/", Verb: "POST", Sort: 305, Group: "模型"},
	{Guid: "navapi-api-model-update", Name: "更新模型", Code: "navapi.model.update", Path: "/models/", Verb: "PUT", Sort: 306, Group: "模型"},
	{Guid: "navapi-api-model-delete", Name: "删除模型", Code: "navapi.model.delete", Path: "/models/:guid", Verb: "DELETE", Sort: 307, Group: "模型"},

	{Guid: "navapi-api-vendor-list", Name: "供应商列表", Code: "navapi.vendor.list", Path: "/vendors/list", Verb: "GET", Sort: 330, Group: "模型"},
	{Guid: "navapi-api-vendor-create", Name: "创建供应商", Code: "navapi.vendor.create", Path: "/vendors/", Verb: "POST", Sort: 331, Group: "模型"},
	{Guid: "navapi-api-vendor-update", Name: "更新供应商", Code: "navapi.vendor.update", Path: "/vendors/", Verb: "PUT", Sort: 332, Group: "模型"},
	{Guid: "navapi-api-vendor-delete", Name: "删除供应商", Code: "navapi.vendor.delete", Path: "/vendors/:id", Verb: "DELETE", Sort: 333, Group: "模型"},

	{Guid: "navapi-api-provider-list", Name: "渠道列表", Code: "navapi.provider.list", Path: "/provider/list", Verb: "GET", Sort: 360, Group: "渠道"},
	{Guid: "navapi-api-provider-test", Name: "测试渠道", Code: "navapi.provider.test", Path: "/provider/test", Verb: "POST", Sort: 361, Group: "渠道"},
	{Guid: "navapi-api-provider-key", Name: "查看渠道密钥", Code: "navapi.provider.key", Path: "/provider/:guid/key", Verb: "GET", Sort: 362, Group: "渠道"},
	{Guid: "navapi-api-provider-key-update", Name: "更新渠道密钥", Code: "navapi.provider.key.update", Path: "/provider/:guid/key", Verb: "PUT", Sort: 363, Group: "渠道"},
	{Guid: "navapi-api-provider-get", Name: "渠道详情", Code: "navapi.provider.get", Path: "/provider/:guid", Verb: "GET", Sort: 364, Group: "渠道"},
	{Guid: "navapi-api-provider-create", Name: "创建渠道", Code: "navapi.provider.create", Path: "/provider/", Verb: "POST", Sort: 365, Group: "渠道"},
	{Guid: "navapi-api-provider-update", Name: "更新渠道", Code: "navapi.provider.update", Path: "/provider/", Verb: "PUT", Sort: 366, Group: "渠道"},
	{Guid: "navapi-api-provider-delete", Name: "删除渠道", Code: "navapi.provider.delete", Path: "/provider/:guid", Verb: "DELETE", Sort: 367, Group: "渠道"},

	{Guid: "navapi-api-usage-self-data", Name: "当前用户用量趋势", Code: "navapi.usage.self.data", Path: "/data/self/list", Verb: "GET", User: true, Sort: 400, Group: "用量"},
	{Guid: "navapi-api-usage-data", Name: "用量趋势", Code: "navapi.usage.data", Path: "/data/list", Verb: "GET", Sort: 401, Group: "用量"},
	{Guid: "navapi-api-usage-self-list", Name: "当前用户用量日志", Code: "navapi.usage.self.list", Path: "/usage/self/list", Verb: "GET", User: true, Sort: 402, Group: "用量"},
	{Guid: "navapi-api-usage-self-stat", Name: "当前用户用量统计", Code: "navapi.usage.self.stat", Path: "/usage/self/stat", Verb: "GET", User: true, Sort: 403, Group: "用量"},
	{Guid: "navapi-api-usage-self-summary", Name: "当前用户用量汇总", Code: "navapi.usage.self.summary", Path: "/usage/self/summary", Verb: "GET", User: true, Sort: 404, Group: "用量"},
	{Guid: "navapi-api-usage-list", Name: "用量日志", Code: "navapi.usage.list", Path: "/usage/list", Verb: "GET", Sort: 405, Group: "用量"},
	{Guid: "navapi-api-usage-stat", Name: "用量统计", Code: "navapi.usage.stat", Path: "/usage/stat", Verb: "GET", Sort: 406, Group: "用量"},
	{Guid: "navapi-api-usage-summary", Name: "用量汇总", Code: "navapi.usage.summary", Path: "/usage/summary", Verb: "GET", Sort: 407, Group: "用量"},

	{Guid: "navapi-api-balance-self", Name: "当前用户余额", Code: "navapi.balance.self", Path: "/balance/self", Verb: "GET", User: true, Sort: 500, Group: "客户"},
	{Guid: "navapi-api-balance-list", Name: "客户余额列表", Code: "navapi.balance.list", Path: "/balance/list", Verb: "GET", Sort: 501, Group: "客户"},
	{Guid: "navapi-api-balance-update", Name: "更新客户余额", Code: "navapi.balance.update", Path: "/balance/", Verb: "PUT", Sort: 502, Group: "客户"},
	{Guid: "navapi-api-user-settings-self", Name: "当前用户设置", Code: "navapi.user.settings.self", Path: "/user-settings/self", Verb: "GET", User: true, Sort: 520, Group: "客户"},
	{Guid: "navapi-api-user-settings-save", Name: "保存当前用户设置", Code: "navapi.user.settings.save", Path: "/user-settings/self", Verb: "PUT", User: true, Sort: 521, Group: "客户"},
	{Guid: "navapi-api-user-settings-get", Name: "用户配置详情", Code: "navapi.user.settings.get", Path: "/user-settings/admin/:userGuid", Verb: "GET", Sort: 522, Group: "客户"},
	{Guid: "navapi-api-user-settings-max-concurrency", Name: "更新用户最大并发", Code: "navapi.user.settings.maxConcurrency", Path: "/user-settings/admin/:userGuid/max-concurrency", Verb: "PUT", Sort: 523, Group: "客户"},

	{Guid: "navapi-api-client-register-settings", Name: "注册设置", Code: "navapi.client.register.settings", Path: "/clients/register/settings", Verb: "GET", Sort: 540, Group: "客户"},
	{Guid: "navapi-api-client-register-save", Name: "保存注册设置", Code: "navapi.client.register.save", Path: "/clients/register/settings", Verb: "PUT", Sort: 541, Group: "客户"},
	{Guid: "navapi-api-client-user-list", Name: "用户列表", Code: "navapi.client.user.list", Path: "/clients/users/list", Verb: "GET", Sort: 542, Group: "客户"},
	{Guid: "navapi-api-client-invite-settings", Name: "邀请设置", Code: "navapi.client.invite.settings", Path: "/clients/invitations/settings", Verb: "GET", Sort: 543, Group: "客户"},
	{Guid: "navapi-api-client-invite-save", Name: "保存邀请设置", Code: "navapi.client.invite.save", Path: "/clients/invitations/settings", Verb: "PUT", Sort: 544, Group: "客户"},
	{Guid: "navapi-api-client-invite-codes", Name: "邀请码列表", Code: "navapi.client.invite.codes", Path: "/clients/invitations/codes", Verb: "GET", Sort: 545, Group: "客户"},
	{Guid: "navapi-api-client-invite-code-create", Name: "创建邀请码", Code: "navapi.client.invite.code.create", Path: "/clients/invitations/code", Verb: "POST", Sort: 546, Group: "客户"},
	{Guid: "navapi-api-client-invite-code-update", Name: "更新邀请码", Code: "navapi.client.invite.code.update", Path: "/clients/invitations/code", Verb: "PUT", Sort: 547, Group: "客户"},
	{Guid: "navapi-api-client-invite-code-get", Name: "邀请码详情", Code: "navapi.client.invite.code.get", Path: "/clients/invitations/code/:id", Verb: "GET", Sort: 548, Group: "客户"},
	{Guid: "navapi-api-client-invite-code-delete", Name: "删除邀请码", Code: "navapi.client.invite.code.delete", Path: "/clients/invitations/code/:id", Verb: "DELETE", Sort: 549, Group: "客户"},
	{Guid: "navapi-api-client-invite-rel", Name: "邀请关系", Code: "navapi.client.invite.relations", Path: "/clients/invitations/relations", Verb: "GET", Sort: 550, Group: "客户"},
	{Guid: "navapi-api-client-invite-stats", Name: "邀请统计", Code: "navapi.client.invite.stats", Path: "/clients/invitations/stats", Verb: "GET", Sort: 551, Group: "客户"},
	{Guid: "navapi-api-client-checkin-settings", Name: "签到设置", Code: "navapi.client.checkin.settings", Path: "/clients/checkin/settings", Verb: "GET", Sort: 552, Group: "客户"},
	{Guid: "navapi-api-client-checkin-save", Name: "保存签到设置", Code: "navapi.client.checkin.save", Path: "/clients/checkin/settings", Verb: "PUT", Sort: 553, Group: "客户"},
	{Guid: "navapi-api-client-checkin-list", Name: "签到记录", Code: "navapi.client.checkin.list", Path: "/clients/checkin/list", Verb: "GET", Sort: 554, Group: "客户"},

	{Guid: "navapi-api-invite-settings", Name: "邀请设置", Code: "navapi.invite.settings", Path: "/invitation/settings", Verb: "GET", Sort: 560, Group: "邀请"},
	{Guid: "navapi-api-invite-save", Name: "保存邀请设置", Code: "navapi.invite.save", Path: "/invitation/settings", Verb: "PUT", Sort: 561, Group: "邀请"},
	{Guid: "navapi-api-invite-codes", Name: "邀请码列表", Code: "navapi.invite.codes", Path: "/invitation/codes", Verb: "GET", Sort: 562, Group: "邀请"},
	{Guid: "navapi-api-invite-code-create", Name: "创建邀请码", Code: "navapi.invite.code.create", Path: "/invitation/code", Verb: "POST", Sort: 563, Group: "邀请"},
	{Guid: "navapi-api-invite-code-update", Name: "更新邀请码", Code: "navapi.invite.code.update", Path: "/invitation/code", Verb: "PUT", Sort: 564, Group: "邀请"},
	{Guid: "navapi-api-invite-code-get", Name: "邀请码详情", Code: "navapi.invite.code.get", Path: "/invitation/code/:id", Verb: "GET", Sort: 565, Group: "邀请"},
	{Guid: "navapi-api-invite-code-delete", Name: "删除邀请码", Code: "navapi.invite.code.delete", Path: "/invitation/code/:id", Verb: "DELETE", Sort: 566, Group: "邀请"},
	{Guid: "navapi-api-invite-relations", Name: "邀请关系列表", Code: "navapi.invite.relations", Path: "/invitation/relations", Verb: "GET", Sort: 567, Group: "邀请"},
	{Guid: "navapi-api-invite-stats", Name: "邀请统计", Code: "navapi.invite.stats", Path: "/invitation/stats", Verb: "GET", Sort: 568, Group: "邀请"},
	{Guid: "navapi-api-invite-self-code", Name: "当前用户邀请码", Code: "navapi.invite.self.code", Path: "/invitation/self/code", Verb: "GET", User: true, Sort: 569, Group: "邀请"},
	{Guid: "navapi-api-invite-self-codes", Name: "当前用户邀请码列表", Code: "navapi.invite.self.codes", Path: "/invitation/self/codes", Verb: "GET", User: true, Sort: 570, Group: "邀请"},
	{Guid: "navapi-api-invite-self-rel", Name: "当前用户邀请关系", Code: "navapi.invite.self.relations", Path: "/invitation/self/relations", Verb: "GET", User: true, Sort: 571, Group: "邀请"},
	{Guid: "navapi-api-invite-self-stats", Name: "当前用户邀请统计", Code: "navapi.invite.self.stats", Path: "/invitation/self/stats", Verb: "GET", User: true, Sort: 572, Group: "邀请"},
	{Guid: "navapi-api-invite-accept", Name: "接受邀请", Code: "navapi.invite.accept", Path: "/invitation/accept", Verb: "POST", User: true, Sort: 573, Group: "邀请"},

	{Guid: "navapi-api-checkin-settings", Name: "签到设置", Code: "navapi.checkin.settings", Path: "/checkin/settings", Verb: "GET", User: true, Sort: 590, Group: "签到"},
	{Guid: "navapi-api-checkin-save", Name: "保存签到设置", Code: "navapi.checkin.save", Path: "/checkin/settings", Verb: "PUT", Sort: 591, Group: "签到"},
	{Guid: "navapi-api-checkin-list", Name: "签到列表", Code: "navapi.checkin.list", Path: "/checkin/list", Verb: "GET", Sort: 592, Group: "签到"},
	{Guid: "navapi-api-checkin-self-list", Name: "当前用户签到记录", Code: "navapi.checkin.self.list", Path: "/checkin/self/list", Verb: "GET", User: true, Sort: 593, Group: "签到"},
	{Guid: "navapi-api-checkin-self-status", Name: "当前用户签到状态", Code: "navapi.checkin.self.status", Path: "/checkin/self/status", Verb: "GET", User: true, Sort: 594, Group: "签到"},
	{Guid: "navapi-api-checkin-self", Name: "当前用户签到", Code: "navapi.checkin.self.create", Path: "/checkin/self", Verb: "POST", User: true, Sort: 595, Group: "签到"},

	{Guid: "navapi-api-task-list", Name: "任务列表", Code: "navapi.task.list", Path: "/task/list", Verb: "GET", Sort: 620, Group: "任务"},
	{Guid: "navapi-api-task-create", Name: "创建任务", Code: "navapi.task.create", Path: "/task/", Verb: "POST", Sort: 621, Group: "任务"},
	{Guid: "navapi-api-task-update", Name: "更新任务", Code: "navapi.task.update", Path: "/task/", Verb: "PUT", Sort: 622, Group: "任务"},
	{Guid: "navapi-api-task-self-list", Name: "当前用户任务列表", Code: "navapi.task.self.list", Path: "/task/self/list", Verb: "GET", User: true, Sort: 623, Group: "任务"},
	{Guid: "navapi-api-task-self-create", Name: "当前用户创建任务", Code: "navapi.task.self.create", Path: "/task/self", Verb: "POST", User: true, Sort: 624, Group: "任务"},
	{Guid: "navapi-api-task-self-update", Name: "当前用户更新任务", Code: "navapi.task.self.update", Path: "/task/self", Verb: "PUT", User: true, Sort: 625, Group: "任务"},
	{Guid: "navapi-api-task-self-get", Name: "当前用户任务详情", Code: "navapi.task.self.get", Path: "/task/self/:task_id", Verb: "GET", User: true, Sort: 626, Group: "任务"},
	{Guid: "navapi-api-task-self-delete", Name: "当前用户删除任务", Code: "navapi.task.self.delete", Path: "/task/self/:task_id", Verb: "DELETE", User: true, Sort: 627, Group: "任务"},
	{Guid: "navapi-api-task-get", Name: "任务详情", Code: "navapi.task.get", Path: "/task/:task_id", Verb: "GET", Sort: 628, Group: "任务"},
	{Guid: "navapi-api-task-delete", Name: "删除任务", Code: "navapi.task.delete", Path: "/task/:task_id", Verb: "DELETE", Sort: 629, Group: "任务"},

	{Guid: "navapi-api-pricing-list", Name: "价格列表", Code: "navapi.pricing.list", Path: "/pricing/list", Verb: "GET", User: true, Sort: 650, Group: "计费"},
	{Guid: "navapi-api-pricing-create", Name: "创建价格", Code: "navapi.pricing.create", Path: "/pricing/", Verb: "POST", Sort: 651, Group: "计费"},
	{Guid: "navapi-api-pricing-update", Name: "更新价格", Code: "navapi.pricing.update", Path: "/pricing/", Verb: "PUT", Sort: 652, Group: "计费"},
	{Guid: "navapi-api-pricing-delete", Name: "删除价格", Code: "navapi.pricing.delete", Path: "/pricing/:id", Verb: "DELETE", Sort: 653, Group: "计费"},

	{Guid: "navapi-api-payment-list", Name: "支付订单列表", Code: "navapi.payment.list", Path: "/payment/list", Verb: "GET", Sort: 670, Group: "支付"},
	{Guid: "navapi-api-payment-self-list", Name: "当前用户支付订单", Code: "navapi.payment.self.list", Path: "/payment/self/list", Verb: "GET", User: true, Sort: 671, Group: "支付"},
	{Guid: "navapi-api-payment-settings", Name: "微信支付设置", Code: "navapi.payment.wechat.settings", Path: "/payment/wechat/settings", Verb: "GET", Sort: 672, Group: "支付"},
	{Guid: "navapi-api-payment-settings-save", Name: "保存微信支付设置", Code: "navapi.payment.wechat.save", Path: "/payment/wechat/settings", Verb: "PUT", Sort: 673, Group: "支付"},
	{Guid: "navapi-api-payment-create", Name: "创建支付订单", Code: "navapi.payment.create", Path: "/payment/create", Verb: "POST", User: true, Sort: 674, Group: "支付"},
	{Guid: "navapi-api-payment-confirm", Name: "确认支付订单", Code: "navapi.payment.confirm", Path: "/payment/confirm", Verb: "POST", Sort: 675, Group: "支付"},
	{Guid: "navapi-api-payment-close", Name: "关闭支付订单", Code: "navapi.payment.close", Path: "/payment/close", Verb: "POST", Sort: 676, Group: "支付"},
	{Guid: "navapi-api-payment-self-close", Name: "当前用户关闭支付订单", Code: "navapi.payment.self.close", Path: "/payment/self/close", Verb: "POST", User: true, Sort: 677, Group: "支付"},
	{Guid: "navapi-api-wallet-self", Name: "当前用户钱包", Code: "navapi.wallet.self", Path: "/wallet/self", Verb: "GET", User: true, Sort: 680, Group: "钱包"},
	{Guid: "navapi-api-wallet-self-records", Name: "当前用户钱包流水", Code: "navapi.wallet.self.records", Path: "/wallet/self/records", Verb: "GET", User: true, Sort: 681, Group: "钱包"},
	{Guid: "navapi-api-wallet-self-activities", Name: "当前用户钱包活动", Code: "navapi.wallet.self.activities", Path: "/wallet/self/activities", Verb: "GET", User: true, Sort: 682, Group: "钱包"},

	{Guid: "navapi-api-sub-plans", Name: "订阅套餐列表", Code: "navapi.subscription.plans", Path: "/subscription/plans", Verb: "GET", User: true, Sort: 700, Group: "订阅"},
	{Guid: "navapi-api-sub-plan-get", Name: "订阅套餐详情", Code: "navapi.subscription.plan.get", Path: "/subscription/plan/:id", Verb: "GET", User: true, Sort: 701, Group: "订阅"},
	{Guid: "navapi-api-sub-plan-create", Name: "创建订阅套餐", Code: "navapi.subscription.plan.create", Path: "/subscription/plan", Verb: "POST", Sort: 702, Group: "订阅"},
	{Guid: "navapi-api-sub-plan-update", Name: "更新订阅套餐", Code: "navapi.subscription.plan.update", Path: "/subscription/plan", Verb: "PUT", Sort: 703, Group: "订阅"},
	{Guid: "navapi-api-sub-plan-delete", Name: "删除订阅套餐", Code: "navapi.subscription.plan.delete", Path: "/subscription/plan/:id", Verb: "DELETE", Sort: 704, Group: "订阅"},
	{Guid: "navapi-api-sub-list", Name: "用户订阅列表", Code: "navapi.subscription.list", Path: "/subscription/list", Verb: "GET", Sort: 705, Group: "订阅"},
	{Guid: "navapi-api-sub-self-list", Name: "当前用户订阅列表", Code: "navapi.subscription.self.list", Path: "/subscription/self/list", Verb: "GET", User: true, Sort: 706, Group: "订阅"},
	{Guid: "navapi-api-sub-subscribe", Name: "订阅套餐", Code: "navapi.subscription.subscribe", Path: "/subscription/subscribe", Verb: "POST", User: true, Sort: 707, Group: "订阅"},

	{Guid: "navapi-api-card-list", Name: "兑换卡列表", Code: "navapi.card.list", Path: "/card/list", Verb: "GET", Sort: 730, Group: "兑换卡"},
	{Guid: "navapi-api-card-stats", Name: "兑换卡统计", Code: "navapi.card.stats", Path: "/card/stats", Verb: "GET", Sort: 731, Group: "兑换卡"},
	{Guid: "navapi-api-card-get", Name: "兑换卡详情", Code: "navapi.card.get", Path: "/card/:guid", Verb: "GET", Sort: 732, Group: "兑换卡"},
	{Guid: "navapi-api-card-create", Name: "创建兑换卡", Code: "navapi.card.create", Path: "/card/", Verb: "POST", Sort: 733, Group: "兑换卡"},
	{Guid: "navapi-api-card-batch", Name: "批量创建兑换卡", Code: "navapi.card.batch", Path: "/card/batch", Verb: "POST", Sort: 734, Group: "兑换卡"},
	{Guid: "navapi-api-card-update", Name: "更新兑换卡", Code: "navapi.card.update", Path: "/card/", Verb: "PUT", Sort: 735, Group: "兑换卡"},
	{Guid: "navapi-api-card-delete", Name: "删除兑换卡", Code: "navapi.card.delete", Path: "/card/:guid", Verb: "DELETE", Sort: 736, Group: "兑换卡"},
	{Guid: "navapi-api-card-redeem", Name: "兑换卡兑换", Code: "navapi.card.redeem", Path: "/card/redeem", Verb: "POST", User: true, Sort: 737, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-list", Name: "兑换码列表", Code: "navapi.redemption.list", Path: "/redemption/list", Verb: "GET", Sort: 740, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-stats", Name: "兑换码统计", Code: "navapi.redemption.stats", Path: "/redemption/stats", Verb: "GET", Sort: 741, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-get", Name: "兑换码详情", Code: "navapi.redemption.get", Path: "/redemption/:guid", Verb: "GET", Sort: 742, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-create", Name: "创建兑换码", Code: "navapi.redemption.create", Path: "/redemption/", Verb: "POST", Sort: 743, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-batch", Name: "批量创建兑换码", Code: "navapi.redemption.batch", Path: "/redemption/batch", Verb: "POST", Sort: 744, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-update", Name: "更新兑换码", Code: "navapi.redemption.update", Path: "/redemption/", Verb: "PUT", Sort: 745, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-delete", Name: "删除兑换码", Code: "navapi.redemption.delete", Path: "/redemption/:guid", Verb: "DELETE", Sort: 746, Group: "兑换卡"},
	{Guid: "navapi-api-redemption-redeem", Name: "兑换码兑换", Code: "navapi.redemption.redeem", Path: "/redemption/redeem", Verb: "POST", User: true, Sort: 747, Group: "兑换卡"},

	{Guid: "navapi-api-announcement-client-list", Name: "客户端公告列表", Code: "navapi.announcement.client.list", Path: "/announcement/client/list", Verb: "GET", User: true, Sort: 779, Group: "运营"},
	{Guid: "navapi-api-announcement-list", Name: "公告列表", Code: "navapi.announcement.list", Path: "/announcement/list", Verb: "GET", Sort: 780, Group: "运营"},
	{Guid: "navapi-api-announcement-get", Name: "公告详情", Code: "navapi.announcement.get", Path: "/announcement/:id", Verb: "GET", Sort: 781, Group: "运营"},
	{Guid: "navapi-api-announcement-create", Name: "创建公告", Code: "navapi.announcement.create", Path: "/announcement/", Verb: "POST", Sort: 782, Group: "运营"},
	{Guid: "navapi-api-announcement-update", Name: "更新公告", Code: "navapi.announcement.update", Path: "/announcement/", Verb: "PUT", Sort: 783, Group: "运营"},
	{Guid: "navapi-api-announcement-id-update", Name: "更新公告", Code: "navapi.announcement.id.update", Path: "/announcement/:id", Verb: "PUT", Sort: 784, Group: "运营"},
	{Guid: "navapi-api-announcement-delete", Name: "删除公告", Code: "navapi.announcement.delete", Path: "/announcement/:id", Verb: "DELETE", Sort: 785, Group: "运营"},

	{Guid: "navapi-api-option-list", Name: "系统配置列表", Code: "navapi.option.list", Path: "/option/list", Verb: "GET", Sort: 820, Group: "系统"},
	{Guid: "navapi-api-option-risk", Name: "风控配置", Code: "navapi.option.risk", Path: "/option/risk_control", Verb: "GET", Sort: 821, Group: "系统"},
	{Guid: "navapi-api-option-risk-save", Name: "保存风控配置", Code: "navapi.option.risk.save", Path: "/option/risk_control", Verb: "PUT", Sort: 822, Group: "系统"},
	{Guid: "navapi-api-option-register", Name: "注册配置", Code: "navapi.option.register", Path: "/option/register_settings", Verb: "GET", Sort: 823, Group: "系统"},
	{Guid: "navapi-api-option-register-save", Name: "保存注册配置", Code: "navapi.option.register.save", Path: "/option/register_settings", Verb: "PUT", Sort: 824, Group: "系统"},
	{Guid: "navapi-api-option-save", Name: "保存系统配置", Code: "navapi.option.save", Path: "/option/", Verb: "PUT", Sort: 825, Group: "系统"},
	{Guid: "navapi-api-option-delete", Name: "删除系统配置", Code: "navapi.option.delete", Path: "/option/:key", Verb: "DELETE", Sort: 826, Group: "系统"},
	{Guid: "navapi-api-setting-list", Name: "设置列表", Code: "navapi.setting.list", Path: "/settings/list", Verb: "GET", Sort: 840, Group: "系统"},
	{Guid: "navapi-api-setting-contact", Name: "联系设置", Code: "navapi.setting.contact", Path: "/settings/contact", Verb: "GET", Sort: 841, Group: "系统"},
	{Guid: "navapi-api-setting-contact-save", Name: "保存联系设置", Code: "navapi.setting.contact.save", Path: "/settings/contact", Verb: "PUT", Sort: 842, Group: "系统"},
	{Guid: "navapi-api-setting-save", Name: "保存设置", Code: "navapi.setting.save", Path: "/settings", Verb: "POST", Sort: 843, Group: "系统"},
	{Guid: "navapi-api-setting-delete", Name: "删除设置", Code: "navapi.setting.delete", Path: "/settings/:guid", Verb: "DELETE", Sort: 844, Group: "系统"},

	{Guid: "navapi-api-message-email-list", Name: "邮箱配置列表", Code: "navapi.message.email.list", Path: "/messages/email-configs/list", Verb: "GET", Sort: 880, Group: "消息"},
	{Guid: "navapi-api-message-email-save", Name: "保存邮箱配置", Code: "navapi.message.email.save", Path: "/messages/email-configs", Verb: "POST", Sort: 881, Group: "消息"},
	{Guid: "navapi-api-message-email-default", Name: "设置默认邮箱", Code: "navapi.message.email.default", Path: "/messages/email-configs/:guid/default", Verb: "POST", Sort: 882, Group: "消息"},
	{Guid: "navapi-api-message-email-debug", Name: "调试发送邮件", Code: "navapi.message.email.debug", Path: "/messages/email-configs/:guid/debug-send", Verb: "POST", Sort: 883, Group: "消息"},
	{Guid: "navapi-api-message-email-delete", Name: "停用邮箱配置", Code: "navapi.message.email.delete", Path: "/messages/email-configs/:guid", Verb: "DELETE", Sort: 884, Group: "消息"},
	{Guid: "navapi-api-message-template-list", Name: "消息模板列表", Code: "navapi.message.template.list", Path: "/messages/templates/list", Verb: "GET", Sort: 890, Group: "消息"},
	{Guid: "navapi-api-message-template-get", Name: "消息模板详情", Code: "navapi.message.template.get", Path: "/messages/templates/:identity", Verb: "GET", Sort: 891, Group: "消息"},
	{Guid: "navapi-api-message-template-save", Name: "保存消息模板", Code: "navapi.message.template.save", Path: "/messages/templates", Verb: "POST", Sort: 892, Group: "消息"},
	{Guid: "navapi-api-message-template-preview", Name: "预览消息模板", Code: "navapi.message.template.preview", Path: "/messages/templates/preview", Verb: "POST", Sort: 893, Group: "消息"},
	{Guid: "navapi-api-message-template-delete", Name: "停用消息模板", Code: "navapi.message.template.delete", Path: "/messages/templates/:guid", Verb: "DELETE", Sort: 894, Group: "消息"},
	{Guid: "navapi-api-message-record-list", Name: "消息发送记录列表", Code: "navapi.message.record.list", Path: "/messages/send-records/list", Verb: "GET", Sort: 900, Group: "消息"},
	{Guid: "navapi-api-message-record-get", Name: "消息发送记录详情", Code: "navapi.message.record.get", Path: "/messages/send-records/:guid", Verb: "GET", Sort: 901, Group: "消息"},
}
