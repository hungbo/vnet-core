package service

type RouteItem struct {
	Name      string      `json:"name"`
	Path      string      `json:"path"`
	Component string      `json:"component,omitempty"`
	Meta      RouteMeta   `json:"meta,omitempty"`
	Props     *bool       `json:"props,omitempty"`
	Redirect  string      `json:"redirect,omitempty"`
	Children  []RouteItem `json:"children,omitempty"`
}

type RouteMeta struct {
	Title              string `json:"title"`
	I18nKey            string `json:"i18nKey,omitempty"`
	Icon               string `json:"icon,omitempty"`
	Order              int    `json:"order,omitempty"`
	HideInMenu         bool   `json:"hideInMenu,omitempty"`
	Constant           bool   `json:"constant,omitempty"`
	KeepAlive          bool   `json:"keepAlive,omitempty"`
	RequiredPermission string `json:"-"`
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
			Meta:      RouteMeta{Title: "vnet_members", I18nKey: "route.vnet_members", RequiredPermission: "members.view"},
		},
		{
			Name:      "vnet_machines",
			Path:      "/vnet/machines",
			Component: "view.vnet_machines",
			Meta:      RouteMeta{Title: "vnet_machines", I18nKey: "route.vnet_machines", RequiredPermission: "machines.view"},
		},

		{
			Name:      "vnet_machine-groups",
			Path:      "/vnet/machine-groups",
			Component: "view.vnet_machine-groups",
			Meta:      RouteMeta{Title: "vnet_machine-groups", I18nKey: "route.vnet_machine-groups", RequiredPermission: "machine_groups.view", Icon: "carbon:data-center"},
		},
		{
			Name:      "vnet_member-groups",
			Path:      "/vnet/member-groups",
			Component: "view.vnet_member-groups",
			Meta:      RouteMeta{Title: "vnet_member-groups", I18nKey: "route.vnet_member-groups", RequiredPermission: "member_groups.view", Icon: "carbon:user-multiple"},
		},
		{
			Name:      "vnet_sessions",
			Path:      "/vnet/sessions",
			Component: "view.vnet_sessions",
			Meta:      RouteMeta{Title: "vnet_sessions", I18nKey: "route.vnet_sessions", RequiredPermission: "sessions.view"},
		},
		{
			Name:      "vnet_orders",
			Path:      "/vnet/orders",
			Component: "view.vnet_orders",
			Meta:      RouteMeta{Title: "vnet_orders", I18nKey: "route.vnet_orders", RequiredPermission: "orders.view"},
		},
		{
			Name:      "vnet_products",
			Path:      "/vnet/products",
			Component: "view.vnet_products",
			Meta:      RouteMeta{Title: "vnet_products", I18nKey: "route.vnet_products", RequiredPermission: "products.view"},
		},
		{
			Name:      "vnet_categories",
			Path:      "/vnet/categories",
			Component: "view.vnet_categories",
			Meta:      RouteMeta{Title: "vnet_categories", I18nKey: "route.vnet_categories", RequiredPermission: "categories.view"},
		},
		{
			Name:      "vnet_suppliers",
			Path:      "/vnet/suppliers",
			Component: "view.vnet_suppliers",
			Meta:      RouteMeta{Title: "vnet_suppliers", I18nKey: "route.vnet_suppliers", RequiredPermission: "suppliers.view"},
		},
		{
			Name:      "vnet_stock-transactions",
			Path:      "/vnet/stock-transactions",
			Component: "view.vnet_stock-transactions",
			Meta:      RouteMeta{Title: "vnet_stock-transactions", I18nKey: "route.vnet_stock-transactions", RequiredPermission: "stock.view"},
		},
		{
			Name:      "vnet_combos",
			Path:      "/vnet/combos",
			Component: "view.vnet_combos",
			Meta:      RouteMeta{Title: "vnet_combos", I18nKey: "route.vnet_combos", RequiredPermission: "combos.view"},
		},
		{
			Name:      "vnet_shifts",
			Path:      "/vnet/shifts",
			Component: "view.vnet_shifts",
			Meta:      RouteMeta{Title: "vnet_shifts", I18nKey: "route.vnet_shifts", RequiredPermission: "shifts.view"},
		},
		{
			Name:      "vnet_bookings",
			Path:      "/vnet/bookings",
			Component: "view.vnet_bookings",
			Meta:      RouteMeta{Title: "vnet_bookings", I18nKey: "route.vnet_bookings", RequiredPermission: "bookings.view"},
		},
		{
			Name:      "vnet_promotions",
			Path:      "/vnet/promotions",
			Component: "view.vnet_promotions",
			Meta:      RouteMeta{Title: "vnet_promotions", I18nKey: "route.vnet_promotions", RequiredPermission: "promotions.view"},
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
			Meta:      RouteMeta{Title: "vnet_transactions", I18nKey: "route.vnet_transactions", RequiredPermission: "transactions.view"},
		},
		{
			Name:      "vnet_settings",
			Path:      "/vnet/settings",
			Component: "view.vnet_settings",
			Meta:      RouteMeta{Title: "vnet_settings", I18nKey: "route.vnet_settings", RequiredPermission: "settings.view"},
		},
		{
			Name:      "vnet_audit",
			Path:      "/vnet/audit",
			Component: "view.vnet_audit",
			Meta:      RouteMeta{Title: "vnet_audit", I18nKey: "route.vnet_audit", RequiredPermission: "audit.view"},
		},
		{
			Name:      "vnet_notifications",
			Path:      "/vnet/notifications",
			Component: "view.vnet_notifications",
			Meta:      RouteMeta{Title: "vnet_notifications", I18nKey: "route.vnet_notifications", RequiredPermission: "notifications.view"},
		},
		{
			Name:      "vnet_feedback",
			Path:      "/vnet/feedback",
			Component: "view.vnet_feedback",
			Meta:      RouteMeta{Title: "vnet_feedback", I18nKey: "route.vnet_feedback", RequiredPermission: "feedback.view"},
		},
		{
			Name:      "vnet_curfew",
			Path:      "/vnet/curfew",
			Component: "view.vnet_curfew",
			Meta:      RouteMeta{Title: "vnet_curfew", I18nKey: "route.vnet_curfew", RequiredPermission: "curfew.view"},
		},
		{
			Name:      "vnet_machine-assets",
			Path:      "/vnet/machine-assets",
			Component: "view.vnet_machine-assets",
			Meta:      RouteMeta{Title: "vnet_machine-assets", I18nKey: "route.vnet_machine-assets", RequiredPermission: "machine_assets.view"},
		},
		{
			Name:      "vnet_app-updates",
			Path:      "/vnet/app-updates",
			Component: "view.vnet_app-updates",
			Meta:      RouteMeta{Title: "vnet_app-updates", I18nKey: "route.vnet_app-updates", RequiredPermission: "app_updates.view"},
		},
		{
			Name:      "vnet_website-blocking",
			Path:      "/vnet/website-blocking",
			Component: "view.vnet_website-blocking",
			Meta:      RouteMeta{Title: "vnet_website-blocking", I18nKey: "route.vnet_website-blocking", RequiredPermission: "website_rules.view"},
		},
		{
			Name:      "vnet_attendance",
			Path:      "/vnet/attendance",
			Component: "view.vnet_attendance",
			Meta:      RouteMeta{Title: "vnet_attendance", I18nKey: "route.vnet_attendance", RequiredPermission: "attendance.view"},
		},
		{
			Name:      "vnet_inventory-counts",
			Path:      "/vnet/inventory-counts",
			Component: "view.vnet_inventory-counts",
			Meta:      RouteMeta{Title: "vnet_inventory-counts", I18nKey: "route.vnet_inventory-counts", RequiredPermission: "inventory_counts.view"},
		},
		{
			Name:      "vnet_cards",
			Path:      "/vnet/cards",
			Component: "view.vnet_cards",
			Meta:      RouteMeta{Title: "vnet_cards", I18nKey: "route.vnet_cards", RequiredPermission: "topup_cards.view"},
		},
		{
			Name:      "vnet_printers",
			Path:      "/vnet/printers",
			Component: "view.vnet_printers",
			Meta:      RouteMeta{Title: "vnet_printers", I18nKey: "route.vnet_printers", RequiredPermission: "printers.view"},
		},
		{
			Name:      "vnet_backups",
			Path:      "/vnet/backups",
			Component: "view.vnet_backups",
			Meta:      RouteMeta{Title: "vnet_backups", I18nKey: "route.vnet_backups", RequiredPermission: "backups.view"},
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

	// Nhóm "Quản lý hệ thống" theo quyền xem tài khoản nhân viên, không còn dựa
	// vào client.admin (mã đó nay chỉ để máy trạm nhận diện, không gác API nào).
	if hasPerm("system.users.view") {
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
