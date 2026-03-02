package tenant

import (
	"context"
	"errors"
)

type key int

const tenantKey key = 0

// ContextWithTenant 将租户ID注入上下文
func ContextWithTenant(ctx context.Context, tenantId string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantId)
}

// TenantFromContext 从上下文获取租户ID
func TenantFromContext(ctx context.Context) (string, error) {
	tid, ok := ctx.Value(tenantKey).(string)
	if !ok {
		return "", errors.New("tenant id not found in context")
	}
	return tid, nil
}

// Manager 租户管理器 (可扩展连接切换等逻辑)
type Manager struct{}

func (m *Manager) GetDBName(tenantId string) string {
	return "db_tenant_" + tenantId
}
