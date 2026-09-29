package admin

import (
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// UserListFilter 管理员用户列表过滤条件。
type UserListFilter struct {
	Query              string
	SubscriptionStatus string
	IdentityProvider   string
}

// CreateUserInput 描述管理员创建普通用户时允许提交的账号与订阅信息。
// 计费模式由后台配置决定，不由请求方传入。
type CreateUserInput struct {
	Username              string
	Password              string
	AvatarURL             string
	DisplayName           string
	Email                 string
	Phone                 string
	Timezone              string
	Locale                string
	SubscriptionTier      string
	SubscriptionExpiresAt *time.Time
}

// CreateUserAuditInput 描述管理员创建用户审计日志输入。
type CreateUserAuditInput struct {
	RequestID     string
	ActorUserID   uint
	CreatedUserID uint
	Username      string
	IP            string
	UserAgent     string
}

// PatchUserByAdminInput 描述管理员局部更新用户请求。
type PatchUserByAdminInput struct {
	RequestID    string
	ActorUserID  uint
	TargetUserID uint
	Patch        PatchUserInput
	IP           string
	UserAgent    string
}

// UpdateUserStatusInput 描述管理员更新用户状态请求。
type UpdateUserStatusInput struct {
	RequestID    string
	ActorUserID  uint
	TargetUserID uint
	Status       string
	Reason       string
	IP           string
	UserAgent    string
}

// ResetUserPasswordByAdminInput 描述管理员重置用户密码请求。
type ResetUserPasswordByAdminInput struct {
	RequestID         string
	ActorUserID       uint
	TargetUserID      uint
	NewPassword       string
	MustResetPassword bool
	IP                string
	UserAgent         string
}

// PatchUserInput 管理员局部更新用户输入。
type PatchUserInput struct {
	AvatarURL             *string
	DisplayName           *string
	Email                 *string
	Phone                 *string
	Role                  *string
	Status                *string
	Timezone              *string
	Locale                *string
	ProfilePreferences    *string
	SubscriptionTier      *string
	SubscriptionExpiresAt *time.Time
	Reason                string
}

// OpenWebUIImportInput 描述 OpenWebUI 用户导入参数。
type OpenWebUIImportInput struct {
	DSN              string
	CreditMultiplier float64
	DryRun           bool
}

// OpenWebUIImportResult 描述 OpenWebUI 用户导入结果。
type OpenWebUIImportResult struct {
	Source                      string
	DedupeField                 string
	DedupeRule                  string
	Scanned                     int
	Imported                    int
	SkippedExistingEmail        int
	SkippedDuplicateSourceEmail int
	SkippedInvalidEmail         int
	SkippedInvalidRow           int
}

// UserResult 用户响应数据（内部传输，不携带序列化标记）。
type UserResult struct {
	User userview.UserView
}

// RevokeUserSessionsResult 管理员吊销用户会话响应数据（内部传输，不携带序列化标记）。
type RevokeUserSessionsResult struct {
	Revoked bool
}

// UpdateUserStatusResult 管理员更新用户状态响应数据（内部传输，不携带序列化标记）。
type UpdateUserStatusResult struct {
	User userview.UserView
}

// ResetUserPasswordResult 管理员重置密码响应数据（内部传输，不携带序列化标记）。
type ResetUserPasswordResult struct {
	Reset bool
}

// DeleteUserResult 管理员删除用户响应数据（内部传输，不携带序列化标记）。
type DeleteUserResult struct {
	Deleted bool
}

// UserAuthEventsResult 用户认证事件分页数据（内部传输，不携带序列化标记）。
type UserAuthEventsResult struct {
	Total   int64
	Results []domainuser.AuthEvent
}

// AuditLogsResult 审计日志分页数据（内部传输，不携带序列化标记）。
type AuditLogsResult struct {
	Total   int64
	Results []domainaudit.Log
}
