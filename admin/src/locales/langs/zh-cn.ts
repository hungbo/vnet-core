// @ts-nocheck
const local: App.I18n.Schema = {
  system: {
    title: 'Soybean 管理系统',
    updateTitle: '系统版本更新通知',
    updateContent: '检测到系统有新版本发布，是否立即刷新页面？',
    updateConfirm: '立即刷新',
    updateCancel: '稍后再说'
  },
  common: {
    action: '操作',
    add: '新增',
    addSuccess: '添加成功',
    backToHome: '返回首页',
    batchDelete: '批量删除',
    cancel: '取消',
    close: '关闭',
    check: '勾选',
    expandColumn: '展开列',
    columnSetting: '列设置',
    config: '配置',
    confirm: '确认',
    delete: '删除',
    deleteSuccess: '删除成功',
    confirmDelete: '确认删除吗？',
    edit: '编辑',
    warning: '警告',
    error: '错误',
    index: '序号',
    keywordSearch: '请输入关键词搜索',
    logout: '退出登录',
    logoutConfirm: '确认退出登录吗？',
    lookForward: '敬请期待',
    modify: '修改',
    modifySuccess: '修改成功',
    noData: '无数据',
    operate: '操作',
    pleaseCheckValue: '请检查输入的值是否合法',
    refresh: '刷新',
    reset: '重置',
    search: '搜索',
    switch: '切换',
    tip: '提示',
    trigger: '触发',
    update: '更新',
    updateSuccess: '更新成功',
    userCenter: '个人中心',
    yesOrNo: {
      yes: '是',
      no: '否'
    }
  },
  request: {
    logout: '请求失败后登出用户',
    logoutMsg: '用户状态失效，请重新登录',
    logoutWithModal: '请求失败后弹出模态框再登出用户',
    logoutWithModalMsg: '用户状态失效，请重新登录',
    refreshToken: '请求的token已过期，刷新token',
    tokenExpired: 'token已过期'
  },
  theme: {
    themeSchema: {
      title: '主题模式',
      light: '亮色模式',
      dark: '暗黑模式',
      auto: '跟随系统'
    },
    grayscale: '灰色模式',
    colourWeakness: '色弱模式',
    layoutMode: {
      title: '布局模式',
      vertical: '左侧菜单模式',
      'vertical-mix': '左侧菜单混合模式',
      horizontal: '顶部菜单模式',
      'horizontal-mix': '顶部菜单混合模式',
      reverseHorizontalMix: '一级菜单与子级菜单位置反转'
    },
    recommendColor: '应用推荐算法的颜色',
    recommendColorDesc: '推荐颜色的算法参照',
    themeColor: {
      title: '主题颜色',
      primary: '主色',
      info: '信息色',
      success: '成功色',
      warning: '警告色',
      error: '错误色',
      followPrimary: '跟随主色'
    },
    scrollMode: {
      title: '滚动模式',
      wrapper: '外层滚动',
      content: '主体滚动'
    },
    page: {
      animate: '页面切换动画',
      mode: {
        title: '页面切换动画类型',
        'fade-slide': '滑动',
        fade: '淡入淡出',
        'fade-bottom': '底部消退',
        'fade-scale': '缩放消退',
        'zoom-fade': '渐变',
        'zoom-out': '闪现',
        none: '无'
      }
    },
    fixedHeaderAndTab: '固定头部和标签栏',
    header: {
      height: '头部高度',
      breadcrumb: {
        visible: '显示面包屑',
        showIcon: '显示面包屑图标'
      },
      multilingual: {
        visible: '显示多语言按钮'
      },
      globalSearch: {
        visible: '显示全局搜索按钮'
      }
    },
    tab: {
      visible: '显示标签栏',
      cache: '标签栏信息缓存',
      height: '标签栏高度',
      mode: {
        title: '标签栏风格',
        chrome: '谷歌风格',
        button: '按钮风格'
      }
    },
    sider: {
      inverted: '深色侧边栏',
      width: '侧边栏宽度',
      collapsedWidth: '侧边栏折叠宽度',
      mixWidth: '混合布局侧边栏宽度',
      mixCollapsedWidth: '混合布局侧边栏折叠宽度',
      mixChildMenuWidth: '混合布局子菜单宽度'
    },
    footer: {
      visible: '显示底部',
      fixed: '固定底部',
      height: '底部高度',
      right: '底部局右'
    },
    watermark: {
      visible: '显示全屏水印',
      text: '水印文本',
      enableUserName: '启用用户名水印'
    },
    themeDrawerTitle: '主题配置',
    pageFunTitle: '页面功能',
    configOperation: {
      copyConfig: '复制配置',
      copySuccessMsg: '复制成功，请替换 src/theme/settings.ts 中的变量 themeSettings',
      resetConfig: '重置配置',
      resetSuccessMsg: '重置成功'
    }
  },
  route: {
    login: '登录',
    403: '无权限',
    404: '页面不存在',
    500: '服务器错误',
    'iframe-page': '外链页面',
    home: '首页',
    document: '文档',
    document_project: '项目文档',
    'document_project-link': '项目文档(外链)',
    document_vue: 'Vue文档',
    document_vite: 'Vite文档',
    document_unocss: 'UnoCSS文档',
    document_naive: 'Naive UI文档',
    document_antd: 'Ant Design Vue文档',
    'document_element-plus': 'Element Plus文档',
    document_alova: 'Alova文档',
    'user-center': '个人中心',
    about: '关于',
    function: '系统功能',
    alova: 'alova示例',
    alova_request: 'alova请求',
    alova_user: '用户列表',
    alova_scenes: '场景化请求',
    function_tab: '标签页',
    'function_multi-tab': '多标签页',
    'function_hide-child': '隐藏子菜单',
    'function_hide-child_one': '隐藏子菜单',
    'function_hide-child_two': '菜单二',
    'function_hide-child_three': '菜单三',
    function_request: '请求',
    'function_toggle-auth': '切换权限',
    'function_super-page': '超级管理员可见',
    system: '系统管理',
    system_user: '用户管理',
    'system_user-detail': '用户详情',
    system_role: '角色管理',
    system_menu: '菜单管理',
    'multi-menu': '多级菜单',
    'multi-menu_first': '菜单一',
    'multi-menu_first_child': '菜单一子菜单',
    'multi-menu_second': '菜单二',
    'multi-menu_second_child': '菜单二子菜单',
    'multi-menu_second_child_home': '菜单二子菜单首页',
    exception: '异常页',
    exception_403: '403',
    exception_404: '404',
    exception_500: '500',
    plugin: '插件示例',
    plugin_copy: '剪贴板',
    plugin_charts: '图表',
    plugin_charts_echarts: 'ECharts',
    plugin_charts_antv: 'AntV',
    plugin_charts_vchart: 'VChart',
    plugin_editor: '编辑器',
    plugin_editor_quill: '富文本编辑器',
    plugin_editor_markdown: 'MD 编辑器',
    plugin_icon: '图标',
    plugin_map: '地图',
    plugin_print: '打印',
    plugin_swiper: 'Swiper',
    plugin_video: '视频',
    plugin_barcode: '条形码',
    plugin_pinyin: '拼音',
    plugin_excel: 'Excel',
    plugin_pdf: 'PDF 预览',
    plugin_gantt: '甘特图',
    plugin_gantt_dhtmlx: 'dhtmlxGantt',
    plugin_gantt_vtable: 'VTableGantt',
    plugin_typeit: '打字机',
    plugin_tables: '表格',
    plugin_tables_vtable: 'VTable',
    vnet: 'VNET',
    vnet_dashboard: 'Dashboard',
    vnet_members: '会员',
    'vnet_members-detail': '会员详情',
    vnet_machines: '机器',
    'vnet_machines-detail': '机器详情',
    'vnet_machine-groups': '机器组',
    'vnet_member-groups': '会员组',
    vnet_sessions: '会话',
    vnet_combos: '套餐',
    vnet_bookings: '预订',
    vnet_promotions: '促销',
    vnet_categories: '分类',
    vnet_products: '产品',
    vnet_orders: '订单',
    vnet_suppliers: '供应商',
    vnet_warehouses: '仓库',
    'vnet_stock-transactions': '库存流水',
    vnet_shifts: '班次',
    vnet_reports: '报告',
    vnet_transactions: '交易',
    vnet_settings: '设置',
    vnet_audit: '审计日志',
    vnet_stores: '门店',
    vnet_backups: '备份',
    vnet_management: '管理',
    vnet_business: '业务',
    vnet_operations: '运营',
    vnet_system: '系统'
  },
  page: {
    login: {
      common: {
        loginOrRegister: '登录 / 注册',
        userNamePlaceholder: '请输入用户名',
        phonePlaceholder: '请输入手机号',
        codePlaceholder: '请输入验证码',
        passwordPlaceholder: '请输入密码',
        confirmPasswordPlaceholder: '请再次输入密码',
        codeLogin: '验证码登录',
        confirm: '确定',
        back: '返回',
        validateSuccess: '验证成功',
        loginSuccess: '登录成功',
        welcomeBack: '欢迎回来，{userName} ！'
      },
      pwdLogin: {
        title: '密码登录',
        rememberMe: '记住我',
        forgetPassword: '忘记密码？',
        register: '注册账号',
        otherAccountLogin: '其他账号登录',
        otherLoginMode: '其他登录方式',
        superAdmin: '超级管理员',
        admin: '管理员',
        user: '普通用户'
      },
      codeLogin: {
        title: '验证码登录',
        getCode: '获取验证码',
        reGetCode: '{time}秒后重新获取',
        sendCodeSuccess: '验证码发送成功',
        imageCodePlaceholder: '请输入图片验证码'
      },
      register: {
        title: '注册账号',
        agreement: '我已经仔细阅读并接受',
        protocol: '《用户协议》',
        policy: '《隐私权政策》'
      },
      resetPwd: {
        title: '重置密码'
      },
      bindWeChat: {
        title: '绑定微信'
      }
    },
    about: {
      title: '关于',
      introduction: `SoybeanAdmin 是一个优雅且功能强大的后台管理模板，基于最新的前端技术栈，包括 Vue3, Vite5, TypeScript, Pinia 和 UnoCSS。它内置了丰富的主题配置和组件，代码规范严谨，实现了自动化的文件路由系统。此外，它还采用了基于 ApiFox 的在线Mock数据方案。SoybeanAdmin 为您提供了一站式的后台管理解决方案，无需额外配置，开箱即用。同样是一个快速学习前沿技术的最佳实践。`,
      projectInfo: {
        title: '项目信息',
        version: '版本',
        latestBuildTime: '最新构建时间',
        githubLink: 'Github 地址',
        previewLink: '预览地址'
      },
      prdDep: '生产依赖',
      devDep: '开发依赖'
    },
    home: {
      branchDesc:
        '为了方便大家开发和更新合并，我们对main分支的代码进行了精简，只保留了首页菜单，其余内容已移至example分支进行维护。预览地址显示的内容即为example分支的内容。',
      greeting: '早安，{userName}, 今天又是充满活力的一天!',
      weatherDesc: '今日多云转晴，20℃ - 25℃!',
      projectCount: '项目数',
      todo: '待办',
      message: '消息',
      downloadCount: '下载量',
      registerCount: '注册量',
      schedule: '作息安排',
      study: '学习',
      work: '工作',
      rest: '休息',
      entertainment: '娱乐',
      visitCount: '访问量',
      turnover: '成交额',
      dealCount: '成交量',
      projectNews: {
        title: '项目动态',
        moreNews: '更多动态',
        desc1: 'Soybean 在2021年5月28日创建了开源项目 soybean-admin!',
        desc2: 'Yanbowe 向 soybean-admin 提交了一个bug，多标签栏不会自适应。',
        desc3: 'Soybean 准备为 soybean-admin 的发布做充分的准备工作!',
        desc4: 'Soybean 正在忙于为soybean-admin写项目说明文档！',
        desc5: 'Soybean 刚才把工作台页面随便写了一些，凑合能看了！'
      },
      creativity: '创意'
    },
    function: {
      tab: {
        tabOperate: {
          title: '标签页操作',
          addTab: '添加标签页',
          addTabDesc: '跳转到关于页面',
          closeTab: '关闭标签页',
          closeCurrentTab: '关闭当前标签页',
          closeAboutTab: '关闭"关于"标签页',
          addMultiTab: '添加多标签页',
          addMultiTabDesc1: '跳转到多标签页页面',
          addMultiTabDesc2: '跳转到多标签页页面(带有查询参数)'
        },
        tabTitle: {
          title: '标签页标题',
          changeTitle: '修改标题',
          change: '修改',
          resetTitle: '重置标题',
          reset: '重置'
        }
      },
      multiTab: {
        routeParam: '路由参数',
        backTab: '返回 function_tab'
      },
      toggleAuth: {
        toggleAccount: '切换账号',
        authHook: '权限钩子函数 `hasAuth`',
        superAdminVisible: '超级管理员可见',
        adminVisible: '管理员可见',
        adminOrUserVisible: '管理员和用户可见'
      },
      request: {
        repeatedErrorOccurOnce: '重复请求错误只出现一次',
        repeatedError: '重复请求错误',
        repeatedErrorMsg1: '自定义请求错误 1',
        repeatedErrorMsg2: '自定义请求错误 2'
      }
    },
    alova: {
      scenes: {
        captchaSend: '发送验证码',
        autoRequest: '自动请求',
        visibilityRequestTips: '浏览器窗口切换自动请求数据',
        pollingRequestTips: '每3秒自动请求一次',
        networkRequestTips: '网络重连后自动请求',
        refreshTime: '更新时间',
        startRequest: '开始请求',
        stopRequest: '停止请求',
        requestCrossComponent: '跨组件触发请求',
        triggerAllRequest: '手动触发所有自动请求'
      }
    },
    manage: {
      common: {
        status: {
          enable: '启用',
          disable: '禁用'
        }
      },
      role: {
        title: '角色列表',
        roleName: '角色名称',
        roleCode: '角色编码',
        roleStatus: '角色状态',
        roleDesc: '角色描述',
        menuAuth: '菜单权限',
        buttonAuth: '按钮权限',
        form: {
          roleName: '请输入角色名称',
          roleCode: '请输入角色编码',
          roleStatus: '请选择角色状态',
          roleDesc: '请输入角色描述'
        },
        addRole: '新增角色',
        editRole: '编辑角色'
      },
      user: {
        title: '用户列表',
        userName: '用户名',
        userGender: '性别',
        nickName: '昵称',
        userPhone: '手机号',
        userEmail: '邮箱',
        userStatus: '用户状态',
        userRole: '用户角色',
        form: {
          userName: '请输入用户名',
          userGender: '请选择性别',
          nickName: '请输入昵称',
          userPhone: '请输入手机号',
          userEmail: '请输入邮箱',
          userStatus: '请选择用户状态',
          userRole: '请选择用户角色'
        },
        addUser: '新增用户',
        editUser: '编辑用户',
        gender: {
          male: '男',
          female: '女'
        }
      },
      menu: {
        home: '首页',
        title: '菜单列表',
        id: 'ID',
        parentId: '父级菜单ID',
        menuType: '菜单类型',
        menuName: '菜单名称',
        routeName: '路由名称',
        routePath: '路由路径',
        pathParam: '路径参数',
        layout: '布局',
        page: '页面组件',
        i18nKey: '国际化key',
        icon: '图标',
        localIcon: '本地图标',
        iconTypeTitle: '图标类型',
        order: '排序',
        constant: '常量路由',
        keepAlive: '缓存路由',
        href: '外链',
        hideInMenu: '隐藏菜单',
        activeMenu: '高亮的菜单',
        multiTab: '支持多页签',
        fixedIndexInTab: '固定在页签中的序号',
        query: '路由参数',
        button: '按钮',
        buttonCode: '按钮编码',
        buttonDesc: '按钮描述',
        menuStatus: '菜单状态',
        form: {
          home: '请选择首页',
          menuType: '请选择菜单类型',
          menuName: '请输入菜单名称',
          routeName: '请输入路由名称',
          routePath: '请输入路由路径',
          pathParam: '请输入路径参数',
          page: '请选择页面组件',
          layout: '请选择布局组件',
          i18nKey: '请输入国际化key',
          icon: '请输入图标',
          localIcon: '请选择本地图标',
          order: '请输入排序',
          keepAlive: '请选择是否缓存路由',
          href: '请输入外链',
          hideInMenu: '请选择是否隐藏菜单',
          activeMenu: '请选择高亮的菜单的路由名称',
          multiTab: '请选择是否支持多标签',
          fixedInTab: '请选择是否固定在页签中',
          fixedIndexInTab: '请输入固定在页签中的序号',
          queryKey: '请输入路由参数Key',
          queryValue: '请输入路由参数Value',
          button: '请选择是否按钮',
          buttonCode: '请输入按钮编码',
          buttonDesc: '请输入按钮描述',
          menuStatus: '请选择菜单状态'
        },
        addMenu: '新增菜单',
        editMenu: '编辑菜单',
        addChildMenu: '新增子菜单',
        type: {
          directory: '目录',
          menu: '菜单'
        },
        iconType: {
          iconify: 'iconify图标',
          local: '本地图标'
        }
      }
    }
  },
  form: {
    required: '不能为空',
    userName: {
      required: '请输入用户名',
      invalid: '用户名格式不正确'
    },
    phone: {
      required: '请输入手机号',
      invalid: '手机号格式不正确'
    },
    pwd: {
      required: '请输入密码',
      invalid: '密码格式不正确，6-18位字符，包含字母、数字、下划线'
    },
    confirmPwd: {
      required: '请输入确认密码',
      invalid: '两次输入密码不一致'
    },
    code: {
      required: '请输入验证码',
      invalid: '验证码格式不正确'
    },
    email: {
      required: '请输入邮箱',
      invalid: '邮箱格式不正确'
    }
  },
  dropdown: {
    closeCurrent: '关闭',
    closeOther: '关闭其它',
    closeLeft: '关闭左侧',
    closeRight: '关闭右侧',
    closeAll: '关闭所有'
  },
  icon: {
    themeConfig: '主题配置',
    themeSchema: '主题模式',
    lang: '切换语言',
    fullscreen: '全屏',
    fullscreenExit: '退出全屏',
    reload: '刷新页面',
    collapse: '折叠菜单',
    expand: '展开菜单',
    pin: '固定',
    unpin: '取消固定'
  },
  datatable: {
    itemCount: '共 {total} 条'
  },
  vnetPages: {
    common: {
      search: '搜索',
      searchPlaceholder: '搜索...',
      add: '新增',
      edit: '编辑',
      delete: '删除',
      detail: '详情',
      action: '操作',
      save: '保存',
      cancel: '取消',
      confirm: '确认',
      status: '状态',
      active: '启用',
      inactive: '禁用',
      yes: '是',
      no: '否',
      back: '返回',
      loading: '加载中...',
      comingSoon: '即将推出',
      success: '成功',
      error: '错误',
      warning: '警告',
      all: '全部'
    },
    dashboard: {
      revenueToday: '今日收入',
      revenueChart: '收入图表',
      activeMachines: '活跃机器',
      machine: '机器',
      member: '会员',
      startTime: '开始时间',
      duration: '时长',
      stats: {
        members: '会员数',
        onlineMachines: '在线机器',
        playing: '游戏中',
        revenueToday: '今日收入'
      }
    },
    machines: {
      title: '机器管理',
      searchPlaceholder: '搜索机器编码/分组',
      add: '添加机器',
      edit: '编辑机器',
      code: '机器编码',
      group: '分组',
      cpu: 'CPU',
      gpu: 'GPU',
      ram: '内存 (GB)',
      disk: '硬盘 (GB)',
      os: '操作系统',
      osPlaceholder: '例如: Windows 11',
      ip: 'IP地址',
      lastHeartbeat: '最后心跳',
      lastSeen: '最后在线',
      createdAt: '创建时间',
      updatedAt: '更新时间',
      detail: '详情',
      statusLabels: {
        offline: '离线',
        available: '可用',
        inUse: '使用中',
        maintenance: '维护中'
      },
      form: {
        code: '机器编码',
        group: '分组',
        cpu: 'CPU',
        gpu: 'GPU',
        ram: '内存 (GB)',
        disk: '硬盘 (GB)',
        os: '操作系统',
        codeRequired: '请输入机器编码',
        groupRequired: '请选择分组'
      },
      messages: {
        addSuccess: '添加机器成功',
        editSuccess: '更新机器成功',
        deleteConfirm: '删除机器 "{code}"？',
        deleteSuccess: '删除成功',
        deleteError: '删除机器失败',
        loadError: '加载数据失败',
        saveError: '保存失败'
      },
      tabs: {
        info: '信息',
        sessions: '会话',
        assets: '资产',
        remote: '远程'
      },
      remote: {
        shutdown: '关机',
        restart: '重启',
        lock: '锁定',
        message: '消息',
        screenshot: '截图',
        sendNotification: '发送通知',
        notificationPlaceholder: '请输入通知内容...',
        send: '发送',
        confirmAction: '在机器 {code} 上执行 "{action}"？',
        actionSent: '已发送 {action} 命令',
        notificationSent: '通知已发送',
        notificationError: '发送通知失败'
      },
      asset: {
        name: '名称',
        type: '类型',
        serial: '序列号',
        status: '状态'
      }
    },
    machineGroups: {
      title: '机器组',
      name: '名称',
      color: '颜色',
      colorPlaceholder: '例如: #1890ff',
      sortOrder: '排序',
      description: '描述',
      create: '创建组',
      edit: '编辑组',
      nameRequired: '请输入组名称',
      createSuccess: '创建组成功',
      editSuccess: '更新组成功',
      deleteConfirm: '删除组 "{name}"？',
      deleteSuccess: '删除成功',
      loadError: '加载数据失败',
      saveError: '保存失败'
    },
    memberGroups: {
      title: '会员组',
      name: '组名',
      minSpent: '最低消费',
      discountPercent: '折扣 %',
      isDefault: '默认',
      createdAt: '创建时间',
      create: '创建组',
      edit: '编辑组',
      nameRequired: '请输入组名',
      createSuccess: '创建组成功',
      editSuccess: '更新组成功',
      deleteConfirm: '删除组 "{name}"？',
      deleteSuccess: '删除成功',
      loadError: '加载数据失败',
      saveError: '保存失败'
    },
    members: {
      title: '会员管理',
      searchPlaceholder: '按姓名/手机号搜索',
      add: '添加会员',
      edit: '编辑会员',
      username: '用户名',
      fullName: '姓名',
      phone: '手机号',
      email: '邮箱',
      balance: '余额',
      bonus: '积分',
      group: '组',
      password: '密码',
      passwordPlaceholder: '留空则不修改',
      isActive: '启用',
      createdAt: '创建时间',
      lastLogin: '最后登录',
      detail: '详情',
      topUp: '充值',
      topUpAmount: '充值金额',
      topUpMethod: '方式',
      resetPassword: '重置密码',
      resetPasswordConfirm: '将生成随机新密码。继续？',
      resetPasswordResult: '会员新密码：',
      copy: '复制',
      copySuccess: '已复制',
      cash: '现金',
      transfer: '转账',
      eWallet: '电子钱包',
      transactions: '交易记录',
      sessions: '上机记录',
      amount: '金额',
      type: '类型',
      method: '方式',
      reference: '参考号',
      form: {
        username: '用户名',
        fullName: '姓名',
        phone: '手机号',
        email: '邮箱',
        password: '密码',
        isActive: '启用',
        usernameRequired: '请输入用户名',
        passwordRequired: '请输入密码',
        amountRequired: '请输入金额',
        methodRequired: '请选择方式'
      },
      messages: {
        addSuccess: '添加会员成功',
        editSuccess: '更新会员成功',
        deleteConfirm: '删除会员 "{name}"？',
        deleteSuccess: '删除成功',
        topUpSuccess: '充值成功',
        topUpError: '充值失败',
        loadError: '加载数据失败',
        saveError: '保存失败',
        loadDetailError: '加载会员信息失败'
      }
    },
    sessions: {
      title: '活跃会话',
      activeSessions: '活跃会话',
      totalActive: '总共: {count} 个活跃会话',
      machineCode: '机器',
      member: '会员',
      startTime: '开始时间',
      duration: '时长',
      endTime: '结束时间',
      status: '状态',
      running: '运行中',
      ended: '已结束',
      end: '结束',
      messages: {
        loadError: '加载数据失败',
        endConfirm: '结束 {member} 在机器 {code} 上的会话？',
        endSuccess: '会话已结束'
      }
    },
    shifts: {
      title: '班次管理',
      openShift: '开班',
      closeShift: '结班',
      handover: '交接',
      startTime: '开始',
      endTime: '结束',
      openingCash: '期初金额',
      closingCash: '期末金额',
      cashIn: '收入',
      cashOut: '支出',
      reason: '原因',
      amount: '金额',
      open: '开启中',
      closed: '已关闭',
      user: '用户',
      form: {
        openingCash: '期初金额',
        closingCash: '期末金额',
        cashIn: '收入',
        cashOut: '支出',
        reason: '原因',
        handoverType: '交接类型',
        amountRequired: '请输入金额',
        amountPlaceholder: '请输入金额',
        handoverTypePlaceholder: '请选择交接类型'
      },
      messages: {
        loadError: '加载数据失败',
        openSuccess: '开班成功',
        openError: '开班失败',
        closeSuccess: '结班成功',
        closeError: '结班失败',
        handoverSuccess: '交接成功',
        handoverError: '交接失败'
      }
    },
    orders: {
      title: '订单管理',
      newOrder: '新订单',
      newOrderMessage: '有新订单需要处理',
      orderCode: '订单号',
      search: '搜索',
      member: '会员',
      total: '总计',
      createdAt: '创建时间',
      completedAt: '完成时间',
      status: '状态',
      note: '备注',
      product: '商品',
      products: '订单商品',
      quantity: '数量',
      unitPrice: '单价',
      subtotal: '小计',
      createOrder: '创建订单',
      selectProduct: '选择商品',
      options: '选项',
      machineCode: '机器',
      operator: '操作人',
      cancel: '取消',
      confirm: '确认',
      pay: '支付',
      previewTotal: '预览总计',
      statusLabels: {
        pending: '待确认',
        confirmed: '已确认',
        completed: '已完成',
        cancelled: '已取消'
      },
      detail: '详情',
      messages: {
        loadError: '加载数据失败',
        loadDetailError: '加载详情失败',
        updateSuccess: '状态更新成功',
        updateError: '状态更新失败',
        paymentConfirm: '确认支付订单 {code}？',
        cancelConfirm: '确认取消订单 {code}？',
        paymentSuccess: '支付成功',
        createSuccess: '订单创建成功',
        createError: '订单创建失败'
      }
    },
    products: {
      title: '商品管理',
      searchPlaceholder: '搜索...',
      add: '添加商品',
      edit: '编辑商品',
      name: '名称',
      category: '分类',
      price: '价格',
      description: '描述',
      image: '图片',
      imageUrl: '图片URL',
      supplier: '供应商',
      isActive: '启用',
      hasStock: '跟踪库存',
      stockQuantity: '数量',
      currentStock: '库存',
      ingredients: '原料',
      unit: '单位',
      minStock: '最低库存',
      pricePerUnit: '单价',
      isRetail: '零售',
      nonRetail: '原料',
      optionGroups: '选项',
      optionName: '选项名称',
      optionIngredientHint: '选择此选项时消耗的原料',
      addOption: '添加选项',
      optionItem: '选项项目',
      stockNote: '备注',
      stockUnitPrice: '单价',
      stockIn: '入库',
      stockOut: '出库',
      stockInDescription: '入库商品',
      stockOutDescription: '出库商品',
      addIngredient: '添加原料...',
      units: {
        piece: '个',
        box: '盒',
        bottle: '瓶',
        can: '罐',
        kg: '公斤',
        liter: '升',
        pack: '包',
        bag: '袋',
        crate: '箱'
      },
      form: {
        name: '名称',
        category: '分类',
        price: '价格',
        nameRequired: '请输入名称',
        priceRequired: '请输入价格',
        categoryPlaceholder: '请选择分类',
        uploadImage: '上传图片',
        note: '备注',
        quantityRequired: '请输入数量'
      },
      messages: {
        addSuccess: '添加成功',
        editSuccess: '更新成功',
        deleteConfirm: '确定删除此商品？',
        deleteSuccess: '删除成功',
        loadError: '加载数据失败',
        saveError: '保存失败',
        addIngredientSuccess: '原料添加成功',
        deleteIngredientSuccess: '原料删除成功',
        stockInSuccess: '入库成功',
        stockOutSuccess: '出库成功',
        noIngredients: '此商品未关联原料'
      }
    },
    suppliers: {
      title: '供应商管理',
      name: '名称',
      phone: '电话',
      email: '邮箱',
      addSupplier: '添加供应商',
      form: {
        nameRequired: '请输入名称'
      },
      messages: {
        addSuccess: '添加成功',
        editSuccess: '更新成功',
        loadError: '加载数据失败',
        saveError: '保存失败'
      }
    },
    warehouses: {
      title: '仓库管理',
      name: '名称',
      address: '地址',
      addWarehouse: '添加仓库',
      form: {
        nameRequired: '请输入名称'
      },
      messages: {
        addSuccess: '添加成功',
        editSuccess: '更新成功',
        loadError: '加载数据失败',
        saveError: '保存失败'
      }
    },
    stockTransactions: {
      title: '库存流水',
      createTransaction: '创建单据',
      type: '类型',
      product: '产品',
      transactionType: '类型',
      importLabel: '入库',
      exportLabel: '出库',
      quantity: '数量',
      unitPrice: '单价',
      totalPrice: '总价',
      before: '之前',
      after: '之后',
      createdAt: '创建时间',
      form: {
        productRequired: '请选择产品',
        quantityRequired: '请输入数量'
      },
      messages: {
        createTransactionSuccess: '创建单据成功',
        loadError: '加载数据失败',
        saveError: '保存失败'
      }
    },
    reports: {
      title: '报表',
      from: '从',
      to: '到',
      dailyRevenue: '日收入',
      date: '日期',
      totalRevenue: '总收入',
      orderCount: '订单数',
      memberCount: '会员数',
      monthlyRevenue: '月收入',
      month: '月份',
      byMember: '按会员',
      memberCode: '会员编码',
      totalSpent: '总消费',
      visitCount: '次数',
      byMachine: '按机器',
      machineName: '机器名称',
      revenue: '收入',
      hours: '小时数',
      sessionCount: '会话数',
      member: '会员',
      machine: '机器',
      messages: {
        loadError: '加载报表失败'
      }
    },
    bookings: {
      title: '预订管理',
      customer: '客户',
      machineCode: '机器',
      from: '从',
      to: '到',
      deposit: '押金',
      checkIn: '签到',
      statusLabels: {
        pending: '待确认',
        confirmed: '已确认',
        checkedIn: '已签到',
        cancelled: '已取消',
        noShow: '未到场'
      },
      messages: {
        loadError: '加载数据失败',
        checkInConfirm: '为 "{customer}" 在机器 {code} 签到？',
        checkInSuccess: '签到成功',
        cancelConfirm: '取消 "{customer}" 的预订？',
        cancelSuccess: '预订已取消'
      }
    },
    categories: {
      title: '分类管理',
      name: '名称',
      icon: '图标',
      order: '排序',
      parent: '父分类',
      searchPlaceholder: '搜索分类...',
      add: '添加分类',
      edit: '编辑分类',
      iconPlaceholder: '选择图标',
      parentPlaceholder: '无（顶级）',
      form: {
        nameRequired: '请输入分类名称'
      },
      messages: {
        addSuccess: '分类添加成功',
        editSuccess: '分类更新成功',
        saveError: '分类保存失败',
        deleteConfirm: '确定删除此分类？',
        deleteSuccess: '分类删除成功'
      }
    },
    promotions: {
      title: '促销活动',
      searchPlaceholder: '搜索促销...',
      add: '添加促销',
      edit: '编辑促销',
      name: '名称',
      type: '类型',
      priority: '优先级',
      isActive: '启用',
      validPeriod: '有效期',
      conditions: '条件',
      rewards: '奖励',
      luckySpinRewards: '幸运转盘奖励',
      value: '值',
      probability: '概率',
      form: {
        name: '名称',
        type: '类型',
        typePlaceholder: '例如: percentage, fixed, combo',
        nameRequired: '请输入名称',
        typeRequired: '请输入类型'
      },
      messages: {
        addSuccess: '促销添加成功',
        editSuccess: '促销更新成功',
        deleteConfirm: '删除促销 "{name}"？',
        deleteSuccess: '删除成功',
        loadError: '加载数据失败',
        loadRewardsError: '加载奖励失败',
        saveError: '保存失败'
      }
    },
    chat: {
      title: '聊天',
      conversations: '对话',
      with: '与',
      newConversation: '新建',
      noMessages: '暂无消息',
      selectConversation: '请选择对话',
      inputPlaceholder: '输入消息...',
      send: '发送',
      createConversation: '新建对话',
      recipients: '接收人',
      selectRecipients: '选择接收人',
      messages: {
        selectAtLeastOne: '请至少选择1人',
        loadConversationsError: '加载对话列表失败',
        loadMessagesError: '加载消息失败',
        sendError: '发送消息失败',
        createSuccess: '创建对话成功',
        createError: '创建对话失败'
      },
      newMessage: '新消息'
    },
    settings: {
      title: '系统设置',
      general: '基本设置',
      billing: '计费设置',
      printing: '打印设置',
      invoice: '发票设置',
      storeName: '门店名称',
      address: '地址',
      phone: '电话',
      email: '邮箱',
      timezone: '时区',
      pricing: '定价',
      pricePerHour: '每小时价格 (VND)',
      pricePerMinute: '每分钟价格 (VND)',
      minMinutes: '最少分钟数',
      hourlyDiscount: '按小时折扣',
      limits: '限制',
      maxBookingsPerDay: '每日最大预订数',
      maxBookingsPerMember: '每会员最大预订数',
      cancelBeforeMinutes: '提前取消(分钟)',
      maxDebt: '最大欠款 (VND)',
      printerType: '打印机类型',
      thermal: '热敏',
      laser: '激光',
      inkjet: '喷墨',
      printerName: '打印机名称',
      paperSize: '纸张大小',
      paperSizePlaceholder: '例如: 80mm',
      autoPrint: '自动打印',
      invoiceTitle: '发票标题',
      invoiceFooter: '发票页脚',
      taxCode: '税号',
      invoiceStartNumber: '起始发票号',
      showLogo: '显示Logo',
      messages: {
        saveSuccess: '设置保存成功',
        saveError: '设置保存失败'
      }
    },
    audit: {
      title: '审计日志',
      action: '操作',
      target: '目标',
      from: '从',
      to: '到',
      id: 'ID',
      user: '用户',
      description: '描述',
      time: '时间',
      actionLabels: {
        create: '创建',
        update: '更新',
        delete: '删除',
        topup: '充值',
        refund: '退款',
        purchase: '购买',
        pay: '支付',
        cancel: '取消'
      },
      entityLabels: {
        member: '会员',
        machine: '机器',
        order: '订单',
        product: '商品',
        category: '分类',
        combo: '套餐',
        promotion: '促销',
        booking: '预订',
        user: '用户',
        role: '角色',
        store: '门店'
      },
      messages: {
        loadError: '加载日志失败'
      }
    },
    stores: {
      title: '门店管理',
      searchPlaceholder: '搜索...',
      add: '添加门店',
      edit: '编辑门店',
      name: '名称',
      code: '编码',
      phone: '电话',
      address: '地址',
      isActive: '启用',
      form: {
        nameRequired: '请输入名称',
        codeRequired: '请输入编码'
      },
      messages: {
        addSuccess: '添加成功',
        editSuccess: '更新成功',
        deleteConfirm: '确定删除此门店？',
        deleteSuccess: '删除成功',
        loadError: '加载数据失败',
        saveError: '保存失败'
      }
    },
    combos: {
      title: '套餐管理',
      searchPlaceholder: '搜索套餐...',
      add: '添加套餐',
      edit: '编辑套餐',
      name: '套餐名称',
      type: '类型',
      fixedSlot: '固定时段',
      prepaid: '预付费',
      price: '价格',
      isActive: '启用',
      from: '从',
      to: '到',
      addSlot: '+ 添加时段',
      minutes: '分钟数',
      form: {
        name: '套餐名称',
        type: '套餐类型',
        price: '价格',
        minutes: '分钟数',
        nameRequired: '请输入套餐名称',
        typeRequired: '请选择类型',
        priceRequired: '请输入价格',
        minutesRequired: '请输入分钟数'
      },
      messages: {
        addSuccess: '套餐添加成功',
        editSuccess: '套餐更新成功',
        deleteConfirm: '删除套餐 "{name}"？',
        deleteSuccess: '删除成功',
        loadError: '加载数据失败',
        saveError: '保存失败'
      }
    },
    backups: {
      title: '备份与恢复',
      createBackup: '创建备份',
      fileName: '文件名',
      size: '大小',
      status: '状态',
      createdAt: '创建时间',
      completed: '已完成',
      running: '运行中',
      failed: '失败',
      restore: '恢复',
      messages: {
        loadError: '加载备份列表失败',
        createConfirm: '创建新备份？此过程可能需要几分钟。',
        creating: '正在创建备份...',
        restoreConfirm: '从备份 "{file}" 恢复数据？当前数据将被替换。',
        restoring: '正在恢复数据...',
        deleteComingSoon: '删除备份功能正在开发中'
      }
    },
    transactions: {
      title: '交易记录',
      from: '从',
      to: '到',
      typePlaceholder: '交易类型',
      all: '全部',
      searchPlaceholder: '搜索姓名/电话/账号',
      date: '时间',
      member: '会员',
      type: '类型',
      amount: '金额',
      balanceBefore: '余额前',
      balanceAfter: '余额后',
      paymentMethod: '方式',
      description: '描述',
      createdBy: '操作人',
      topup: '充值',
      sessionFee: '上机费',
      refund: '退款',
      cancel: '取消',
      comboPurchase: '购买套餐',
      messages: {
        loadError: '加载交易记录失败'
      }
    }
  }
};

export default local;
