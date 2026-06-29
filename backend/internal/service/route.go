package service

type RouteItem struct {
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	Component string     `json:"component,omitempty"`
	Meta      RouteMeta  `json:"meta,omitempty"`
	Props     *bool      `json:"props,omitempty"`
	Redirect  string     `json:"redirect,omitempty"`
	Children  []RouteItem `json:"children,omitempty"`
}

type RouteMeta struct {
	Title              string   `json:"title"`
	I18nKey            string   `json:"i18nKey,omitempty"`
	Icon               string   `json:"icon,omitempty"`
	Order              int      `json:"order,omitempty"`
	HideInMenu         bool     `json:"hideInMenu,omitempty"`
	Constant           bool     `json:"constant,omitempty"`
	KeepAlive          bool     `json:"keepAlive,omitempty"`
	RequiredPermission string   `json:"-"`
}

type UserRoutesResponse struct {
	Routes []RouteItem `json:"routes"`
	Home   string      `json:"home"`
}

type RouteService struct{}

func NewRouteService() *RouteService {
	return &RouteService{}
}

func (s *RouteService) GetConstantRoutes() []RouteItem {
	return []RouteItem{
		{
			Name:      "403",
			Path:      "/403",
			Component: "layout.blank$view.403",
			Meta:      RouteMeta{Title: "403", I18nKey: "route.403", Constant: true, HideInMenu: true},
		},
		{
			Name:      "404",
			Path:      "/404",
			Component: "layout.blank$view.404",
			Meta:      RouteMeta{Title: "404", I18nKey: "route.404", Constant: true, HideInMenu: true},
		},
		{
			Name:      "500",
			Path:      "/500",
			Component: "layout.blank$view.500",
			Meta:      RouteMeta{Title: "500", I18nKey: "route.500", Constant: true, HideInMenu: true},
		},
		{
			Name:      "login",
			Path:      "/login/:module(pwd-login)?",
			Component: "layout.blank$view.login",
			Meta:      RouteMeta{Title: "login", I18nKey: "route.login", Constant: true, HideInMenu: true},
		},
		{
			Name:      "iframe-page",
			Path:      "/iframe-page/:url",
			Component: "layout.base$view.iframe-page",
			Props:     boolPtr(true),
			Meta:      RouteMeta{Title: "iframe-page", I18nKey: "route.iframe-page", Constant: true, HideInMenu: true, KeepAlive: true},
		},
		{
			Name:      "user-center",
			Path:      "/user-center",
			Component: "layout.base$view.user-center",
			Meta:      RouteMeta{Title: "user-center", I18nKey: "route.user-center", HideInMenu: true},
		},
	}
}

func (s *RouteService) GetUserRoutes(permissions []string) UserRoutesResponse {
	hasPerm := func(perm string) bool {
		if perm == "" {
			return true
		}
		for _, p := range permissions {
			if p == "*" || p == perm {
				return true
			}
		}
		return false
	}

	allVnetRoutes := []RouteItem{
		{
			Name:      "vnet_dashboard",
			Path:      "/vnet/dashboard",
			Component: "view.vnet_dashboard",
			Meta:      RouteMeta{Title: "vnet_dashboard", I18nKey: "route.vnet_dashboard"},
		},
		{
			Name:      "vnet_members",
			Path:      "/vnet/members",
			Component: "view.vnet_members",
			Meta:      RouteMeta{Title: "vnet_members", I18nKey: "route.vnet_members"},
		},
		{
			Name:      "vnet_machines",
			Path:      "/vnet/machines",
			Component: "view.vnet_machines",
			Meta:      RouteMeta{Title: "vnet_machines", I18nKey: "route.vnet_machines"},
		},

		{
			Name:      "vnet_machines-detail",
			Path:      "/vnet/machines/:id",
			Component: "view.vnet_machines-detail",
			Meta:      RouteMeta{Title: "vnet_machines-detail", I18nKey: "route.vnet_machines-detail", HideInMenu: true},
		},
		{
			Name:      "vnet_machine-groups",
			Path:      "/vnet/machine-groups",
			Component: "view.vnet_machine-groups",
			Meta:      RouteMeta{Title: "vnet_machine-groups", I18nKey: "route.vnet_machine-groups", Icon: "carbon:data-center"},
		},
		{
			Name:      "vnet_member-groups",
			Path:      "/vnet/member-groups",
			Component: "view.vnet_member-groups",
			Meta:      RouteMeta{Title: "vnet_member-groups", I18nKey: "route.vnet_member-groups", Icon: "carbon:user-multiple"},
		},
		{
			Name:      "vnet_sessions",
			Path:      "/vnet/sessions",
			Component: "view.vnet_sessions",
			Meta:      RouteMeta{Title: "vnet_sessions", I18nKey: "route.vnet_sessions"},
		},
		{
			Name:      "vnet_orders",
			Path:      "/vnet/orders",
			Component: "view.vnet_orders",
			Meta:      RouteMeta{Title: "vnet_orders", I18nKey: "route.vnet_orders"},
		},
		{
			Name:      "vnet_products",
			Path:      "/vnet/products",
			Component: "view.vnet_products",
			Meta:      RouteMeta{Title: "vnet_products", I18nKey: "route.vnet_products"},
		},
		{
			Name:      "vnet_categories",
			Path:      "/vnet/categories",
			Component: "view.vnet_categories",
			Meta:      RouteMeta{Title: "vnet_categories", I18nKey: "route.vnet_categories"},
		},
		{
			Name:      "vnet_suppliers",
			Path:      "/vnet/suppliers",
			Component: "view.vnet_suppliers",
			Meta:      RouteMeta{Title: "vnet_suppliers", I18nKey: "route.vnet_suppliers"},
		},
		{
			Name:      "vnet_warehouses",
			Path:      "/vnet/warehouses",
			Component: "view.vnet_warehouses",
			Meta:      RouteMeta{Title: "vnet_warehouses", I18nKey: "route.vnet_warehouses"},
		},
		{
			Name:      "vnet_stock-transactions",
			Path:      "/vnet/stock-transactions",
			Component: "view.vnet_stock-transactions",
			Meta:      RouteMeta{Title: "vnet_stock-transactions", I18nKey: "route.vnet_stock-transactions"},
		},
		{
			Name:      "vnet_combos",
			Path:      "/vnet/combos",
			Component: "view.vnet_combos",
			Meta:      RouteMeta{Title: "vnet_combos", I18nKey: "route.vnet_combos"},
		},
		{
			Name:      "vnet_shifts",
			Path:      "/vnet/shifts",
			Component: "view.vnet_shifts",
			Meta:      RouteMeta{Title: "vnet_shifts", I18nKey: "route.vnet_shifts"},
		},
		{
			Name:      "vnet_bookings",
			Path:      "/vnet/bookings",
			Component: "view.vnet_bookings",
			Meta:      RouteMeta{Title: "vnet_bookings", I18nKey: "route.vnet_bookings"},
		},
		{
			Name:      "vnet_promotions",
			Path:      "/vnet/promotions",
			Component: "view.vnet_promotions",
			Meta:      RouteMeta{Title: "vnet_promotions", I18nKey: "route.vnet_promotions"},
		},
		{
			Name:      "vnet_reports",
			Path:      "/vnet/reports",
			Component: "view.vnet_reports",
			Meta:      RouteMeta{Title: "vnet_reports", I18nKey: "route.vnet_reports", RequiredPermission: "reports.view"},
		},
		{
			Name:      "vnet_transactions",
			Path:      "/vnet/transactions",
			Component: "view.vnet_transactions",
			Meta:      RouteMeta{Title: "vnet_transactions", I18nKey: "route.vnet_transactions"},
		},
		{
			Name:      "vnet_settings",
			Path:      "/vnet/settings",
			Component: "view.vnet_settings",
			Meta:      RouteMeta{Title: "vnet_settings", I18nKey: "route.vnet_settings", RequiredPermission: "settings.edit"},
		},
		{
			Name:      "vnet_audit",
			Path:      "/vnet/audit",
			Component: "view.vnet_audit",
			Meta:      RouteMeta{Title: "vnet_audit", I18nKey: "route.vnet_audit", RequiredPermission: "client.admin"},
		},
		{
			Name:      "vnet_stores",
			Path:      "/vnet/stores",
			Component: "view.vnet_stores",
			Meta:      RouteMeta{Title: "vnet_stores", I18nKey: "route.vnet_stores", RequiredPermission: "client.admin"},
		},
		{
			Name:      "vnet_backups",
			Path:      "/vnet/backups",
			Component: "view.vnet_backups",
			Meta:      RouteMeta{Title: "vnet_backups", I18nKey: "route.vnet_backups", RequiredPermission: "client.admin"},
		},
	}

	var filteredVnetChildren []RouteItem
	for _, r := range allVnetRoutes {
		if hasPerm(r.Meta.RequiredPermission) {
			r.Meta.RequiredPermission = ""
			filteredVnetChildren = append(filteredVnetChildren, r)
		}
	}

	vnetRoute := RouteItem{
		Name:      "vnet",
		Path:      "/vnet",
		Component: "layout.base",
		Meta:      RouteMeta{Title: "vnet", I18nKey: "route.vnet", Order: 1, Icon: "ant-design:appstore-outlined"},
		Children:  filteredVnetChildren,
	}

	routes := []RouteItem{vnetRoute}

		if hasPerm("client.admin") {
			routes = append(routes, RouteItem{
				Name:      "system",
				Path:      "/system",
				Component: "layout.base",
				Meta:      RouteMeta{Title: "system", I18nKey: "route.system", Order: 9, Icon: "carbon:cloud-service-management"},
				Children: []RouteItem{
					{
						Name:      "system_user",
						Path:      "/system/user",
						Component: "view.system_user",
						Meta:      RouteMeta{Title: "system_user", I18nKey: "route.system_user", Order: 1, Icon: "carbon:user-admin"},
					},
					{
						Name:      "system_role",
						Path:      "/system/role",
						Component: "view.system_role",
						Meta:      RouteMeta{Title: "system_role", I18nKey: "route.system_role", Order: 2, Icon: "carbon:user-role"},
					},
					{
						Name:      "system_menu",
						Path:      "/system/menu",
						Component: "view.system_menu",
						Meta:      RouteMeta{Title: "system_menu", I18nKey: "route.system_menu", Order: 3, Icon: "carbon:tree-view"},
					},
					{
						Name:      "system_user-detail",
						Path:      "/system/user-detail/:id",
						Component: "view.system_user-detail",
						Meta:      RouteMeta{Title: "system_user-detail", I18nKey: "route.system_user-detail", HideInMenu: true},
					},
				},
			})
		}

	return UserRoutesResponse{
		Routes: routes,
		Home:   "vnet_dashboard",
	}
}

func boolPtr(b bool) *bool {
	return &b
}
