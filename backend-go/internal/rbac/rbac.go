package rbac

import (
	"slices"
)

const (
	RoleAdmin            = "Admin"
	RoleFleetManager     = "Fleet Manager"
	RoleDriver           = "Driver"
	RoleSafetyOfficer    = "Safety Officer"
	RoleFinancialAnalyst = "Financial Analyst"
)

var AllRoles = []string{
	RoleAdmin,
	RoleFleetManager,
	RoleDriver,
	RoleSafetyOfficer,
	RoleFinancialAnalyst,
}

var AnyAuthenticated = []string{
	RoleAdmin,
	RoleFleetManager,
	RoleDriver,
	RoleSafetyOfficer,
	RoleFinancialAnalyst,
}

type RouteDefinition struct {
	Method      string
	Path        string
	Roles       []string
	IsPublic    bool
	Description string
}

type Registry struct {
	routes []RouteDefinition
}

func NewRegistry() *Registry {
	return &Registry{
		routes: make([]RouteDefinition, 0),
	}
}

func (r *Registry) Register(method, path string, roles []string, isPublic bool, description string) RouteDefinition {
	def := RouteDefinition{
		Method:      method,
		Path:        path,
		Roles:       roles,
		IsPublic:    isPublic,
		Description: description,
	}
	r.routes = append(r.routes, def)
	return def
}

func (r *Registry) Routes() []RouteDefinition {
	copied := make([]RouteDefinition, len(r.routes))
	copy(copied, r.routes)
	return copied
}

func IsAuthorized(userRole string, allowedRoles []string) bool {
	if userRole == RoleAdmin {
		return true
	}
	return slices.Contains(allowedRoles, userRole)
}
