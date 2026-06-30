// @ts-nocheck
const local: App.I18n.Schema = {
  system: {
    title: 'SoybeanAdmin',
    updateTitle: 'System Version Update Notification',
    updateContent: 'A new version of the system has been detected. Do you want to refresh the page immediately?',
    updateConfirm: 'Refresh immediately',
    updateCancel: 'Later'
  },
  common: {
    action: 'Action',
    add: 'Add',
    addSuccess: 'Add Success',
    backToHome: 'Back to home',
    batchDelete: 'Batch Delete',
    cancel: 'Cancel',
    close: 'Close',
    check: 'Check',
    expandColumn: 'Expand Column',
    columnSetting: 'Column Setting',
    config: 'Config',
    confirm: 'Confirm',
    delete: 'Delete',
    deleteSuccess: 'Delete Success',
    confirmDelete: 'Are you sure you want to delete?',
    edit: 'Edit',
    warning: 'Warning',
    error: 'Error',
    index: 'Index',
    keywordSearch: 'Please enter keyword',
    logout: 'Logout',
    logoutConfirm: 'Are you sure you want to log out?',
    lookForward: 'Coming soon',
    modify: 'Modify',
    modifySuccess: 'Modify Success',
    noData: 'No Data',
    operate: 'Operate',
    pleaseCheckValue: 'Please check whether the value is valid',
    refresh: 'Refresh',
    reset: 'Reset',
    search: 'Search',
    switch: 'Switch',
    tip: 'Tip',
    trigger: 'Trigger',
    update: 'Update',
    updateSuccess: 'Update Success',
    userCenter: 'User Center',
    yesOrNo: {
      yes: 'Yes',
      no: 'No'
    }
  },
  request: {
    logout: 'Logout user after request failed',
    logoutMsg: 'User status is invalid, please log in again',
    logoutWithModal: 'Pop up modal after request failed and then log out user',
    logoutWithModalMsg: 'User status is invalid, please log in again',
    refreshToken: 'The requested token has expired, refresh the token',
    tokenExpired: 'The requested token has expired'
  },
  theme: {
    themeSchema: {
      title: 'Theme Schema',
      light: 'Light',
      dark: 'Dark',
      auto: 'Follow System'
    },
    grayscale: 'Grayscale',
    colourWeakness: 'Colour Weakness',
    layoutMode: {
      title: 'Layout Mode',
      vertical: 'Vertical Menu Mode',
      horizontal: 'Horizontal Menu Mode',
      'vertical-mix': 'Vertical Mix Menu Mode',
      'horizontal-mix': 'Horizontal Mix menu Mode',
      reverseHorizontalMix: 'Reverse first level menus and child level menus position'
    },
    recommendColor: 'Apply Recommended Color Algorithm',
    recommendColorDesc: 'The recommended color algorithm refers to',
    themeColor: {
      title: 'Theme Color',
      primary: 'Primary',
      info: 'Info',
      success: 'Success',
      warning: 'Warning',
      error: 'Error',
      followPrimary: 'Follow Primary'
    },
    scrollMode: {
      title: 'Scroll Mode',
      wrapper: 'Wrapper',
      content: 'Content'
    },
    page: {
      animate: 'Page Animate',
      mode: {
        title: 'Page Animate Mode',
        fade: 'Fade',
        'fade-slide': 'Slide',
        'fade-bottom': 'Fade Zoom',
        'fade-scale': 'Fade Scale',
        'zoom-fade': 'Zoom Fade',
        'zoom-out': 'Zoom Out',
        none: 'None'
      }
    },
    fixedHeaderAndTab: 'Fixed Header And Tab',
    header: {
      height: 'Header Height',
      breadcrumb: {
        visible: 'Breadcrumb Visible',
        showIcon: 'Breadcrumb Icon Visible'
      },
      multilingual: {
        visible: 'Display multilingual button'
      },
      globalSearch: {
        visible: 'Display global search button'
      }
    },
    tab: {
      visible: 'Tab Visible',
      cache: 'Tag Bar Info Cache',
      height: 'Tab Height',
      mode: {
        title: 'Tab Mode',
        chrome: 'Chrome',
        button: 'Button'
      }
    },
    sider: {
      inverted: 'Dark Sider',
      width: 'Sider Width',
      collapsedWidth: 'Sider Collapsed Width',
      mixWidth: 'Mix Sider Width',
      mixCollapsedWidth: 'Mix Sider Collapse Width',
      mixChildMenuWidth: 'Mix Child Menu Width'
    },
    footer: {
      visible: 'Footer Visible',
      fixed: 'Fixed Footer',
      height: 'Footer Height',
      right: 'Right Footer'
    },
    watermark: {
      visible: 'Watermark Full Screen Visible',
      text: 'Watermark Text',
      enableUserName: 'Enable User Name Watermark'
    },
    themeDrawerTitle: 'Theme Configuration',
    pageFunTitle: 'Page Function',
    configOperation: {
      copyConfig: 'Copy Config',
      copySuccessMsg: 'Copy Success, Please replace the variable "themeSettings" in "src/theme/settings.ts"',
      resetConfig: 'Reset Config',
      resetSuccessMsg: 'Reset Success'
    }
  },
  route: {
    login: 'Login',
    403: 'No Permission',
    404: 'Page Not Found',
    500: 'Server Error',
    'iframe-page': 'Iframe',
    home: 'Home',
    document: 'Document',
    document_project: 'Project Document',
    'document_project-link': 'Project Document(External Link)',
    document_vue: 'Vue Document',
    document_vite: 'Vite Document',
    document_unocss: 'UnoCSS Document',
    document_naive: 'Naive UI Document',
    document_antd: 'Ant Design Vue Document',
    'document_element-plus': 'Element Plus Document',
    document_alova: 'Alova Document',
    'user-center': 'User Center',
    about: 'About',
    function: 'System Function',
    alova: 'Alova Example',
    alova_request: 'Alova Request',
    alova_user: 'User List',
    alova_scenes: 'Scenario Request',
    function_tab: 'Tab',
    'function_multi-tab': 'Multi Tab',
    'function_hide-child': 'Hide Child',
    'function_hide-child_one': 'Hide Child',
    'function_hide-child_two': 'Two',
    'function_hide-child_three': 'Three',
    function_request: 'Request',
    'function_toggle-auth': 'Toggle Auth',
    'function_super-page': 'Super Admin Visible',
    system: 'System Manage',
    system_user: 'User Manage',
    'system_user-detail': 'User Detail',
    system_role: 'Role Manage',
    system_menu: 'Menu Manage',
    'multi-menu': 'Multi Menu',
    'multi-menu_first': 'Menu One',
    'multi-menu_first_child': 'Menu One Child',
    'multi-menu_second': 'Menu Two',
    'multi-menu_second_child': 'Menu Two Child',
    'multi-menu_second_child_home': 'Menu Two Child Home',
    exception: 'Exception',
    exception_403: '403',
    exception_404: '404',
    exception_500: '500',
    plugin: 'Plugin',
    plugin_copy: 'Copy',
    plugin_charts: 'Charts',
    plugin_charts_echarts: 'ECharts',
    plugin_charts_antv: 'AntV',
    plugin_charts_vchart: 'VChart',
    plugin_editor: 'Editor',
    plugin_editor_quill: 'Quill',
    plugin_editor_markdown: 'Markdown',
    plugin_icon: 'Icon',
    plugin_map: 'Map',
    plugin_print: 'Print',
    plugin_swiper: 'Swiper',
    plugin_video: 'Video',
    plugin_barcode: 'Barcode',
    plugin_pinyin: 'pinyin',
    plugin_excel: 'Excel',
    plugin_pdf: 'PDF preview',
    plugin_gantt: 'Gantt Chart',
    plugin_gantt_dhtmlx: 'dhtmlxGantt',
    plugin_gantt_vtable: 'VTableGantt',
    plugin_typeit: 'Typeit',
    plugin_tables: 'Tables',
    plugin_tables_vtable: 'VTable',
    vnet: 'VNET',
    vnet_dashboard: 'Dashboard',
    vnet_members: 'Members',
    'vnet_members-detail': 'Member Detail',
    vnet_machines: 'Machines',
    'vnet_machine-groups': 'Machine Groups',
    'vnet_member-groups': 'Member Groups',
    vnet_sessions: 'Sessions',
    vnet_combos: 'Combos',
    vnet_bookings: 'Bookings',
    vnet_promotions: 'Promotions',
    vnet_categories: 'Categories',
    vnet_products: 'Products',
    vnet_orders: 'Orders',
    vnet_suppliers: 'Suppliers',
    vnet_warehouses: 'Warehouses',
    'vnet_stock-transactions': 'Stock',
    vnet_shifts: 'Shifts',
    vnet_reports: 'Reports',
    vnet_transactions: 'Transactions',
    vnet_settings: 'Settings',
    vnet_audit: 'Audit Logs',
    vnet_backups: 'Backups',
    vnet_management: 'Management',
    vnet_business: 'Business',
    vnet_operations: 'Operations',
    vnet_system: 'System'
  },
  page: {
    login: {
      common: {
        loginOrRegister: 'Login / Register',
        userNamePlaceholder: 'Please enter user name',
        phonePlaceholder: 'Please enter phone number',
        codePlaceholder: 'Please enter verification code',
        passwordPlaceholder: 'Please enter password',
        confirmPasswordPlaceholder: 'Please enter password again',
        codeLogin: 'Verification code login',
        confirm: 'Confirm',
        back: 'Back',
        validateSuccess: 'Verification passed',
        loginSuccess: 'Login successfully',
        welcomeBack: 'Welcome back, {userName} !'
      },
      pwdLogin: {
        title: 'Password Login',
        rememberMe: 'Remember me',
        forgetPassword: 'Forget password?',
        register: 'Register',
        otherAccountLogin: 'Other Account Login',
        otherLoginMode: 'Other Login Mode',
        superAdmin: 'Super Admin',
        admin: 'Admin',
        user: 'User'
      },
      codeLogin: {
        title: 'Verification Code Login',
        getCode: 'Get verification code',
        reGetCode: 'Reacquire after {time}s',
        sendCodeSuccess: 'Verification code sent successfully',
        imageCodePlaceholder: 'Please enter image verification code'
      },
      register: {
        title: 'Register',
        agreement: 'I have read and agree to',
        protocol: '《User Agreement》',
        policy: '《Privacy Policy》'
      },
      resetPwd: {
        title: 'Reset Password'
      },
      bindWeChat: {
        title: 'Bind WeChat'
      }
    },
    about: {
      title: 'About',
      introduction: `SoybeanAdmin is an elegant and powerful admin template, based on the latest front-end technology stack, including Vue3, Vite5, TypeScript, Pinia and UnoCSS. It has built-in rich theme configuration and components, strict code specifications, and an automated file routing system. In addition, it also uses the online mock data solution based on ApiFox. SoybeanAdmin provides you with a one-stop admin solution, no additional configuration, and out of the box. It is also a best practice for learning cutting-edge technologies quickly.`,
      projectInfo: {
        title: 'Project Info',
        version: 'Version',
        latestBuildTime: 'Latest Build Time',
        githubLink: 'Github Link',
        previewLink: 'Preview Link'
      },
      prdDep: 'Production Dependency',
      devDep: 'Development Dependency'
    },
    home: {
      branchDesc:
        'For the convenience of everyone in developing and updating the merge, we have streamlined the code of the main branch, only retaining the homepage menu, and the rest of the content has been moved to the example branch for maintenance. The preview address displays the content of the example branch.',
      greeting: 'Good morning, {userName}, today is another day full of vitality!',
      weatherDesc: 'Today is cloudy to clear, 20℃ - 25℃!',
      projectCount: 'Project Count',
      todo: 'Todo',
      message: 'Message',
      downloadCount: 'Download Count',
      registerCount: 'Register Count',
      schedule: 'Work and rest Schedule',
      study: 'Study',
      work: 'Work',
      rest: 'Rest',
      entertainment: 'Entertainment',
      visitCount: 'Visit Count',
      turnover: 'Turnover',
      dealCount: 'Deal Count',
      projectNews: {
        title: 'Project News',
        moreNews: 'More News',
        desc1: 'Soybean created the open source project soybean-admin on May 28, 2021!',
        desc2: 'Yanbowe submitted a bug to soybean-admin, the multi-tab bar will not adapt.',
        desc3: 'Soybean is ready to do sufficient preparation for the release of soybean-admin!',
        desc4: 'Soybean is busy writing project documentation for soybean-admin!',
        desc5: 'Soybean just wrote some of the workbench pages casually, and it was enough to see!'
      },
      creativity: 'Creativity'
    },
    function: {
      tab: {
        tabOperate: {
          title: 'Tab Operation',
          addTab: 'Add Tab',
          addTabDesc: 'To about page',
          closeTab: 'Close Tab',
          closeCurrentTab: 'Close Current Tab',
          closeAboutTab: 'Close "About" Tab',
          addMultiTab: 'Add Multi Tab',
          addMultiTabDesc1: 'To MultiTab page',
          addMultiTabDesc2: 'To MultiTab page(with query params)'
        },
        tabTitle: {
          title: 'Tab Title',
          changeTitle: 'Change Title',
          change: 'Change',
          resetTitle: 'Reset Title',
          reset: 'Reset'
        }
      },
      multiTab: {
        routeParam: 'Route Param',
        backTab: 'Back function_tab'
      },
      toggleAuth: {
        toggleAccount: 'Toggle Account',
        authHook: 'Auth Hook Function `hasAuth`',
        superAdminVisible: 'Super Admin Visible',
        adminVisible: 'Admin Visible',
        adminOrUserVisible: 'Admin and User Visible'
      },
      request: {
        repeatedErrorOccurOnce: 'Repeated Request Error Occurs Once',
        repeatedError: 'Repeated Request Error',
        repeatedErrorMsg1: 'Custom Request Error 1',
        repeatedErrorMsg2: 'Custom Request Error 2'
      }
    },
    alova: {
      scenes: {
        captchaSend: 'Captcha Send',
        autoRequest: 'Auto Request',
        visibilityRequestTips: 'Automatically request when switching browser window',
        pollingRequestTips: 'It will request every 3 seconds',
        networkRequestTips: 'Automatically request after network reconnecting',
        refreshTime: 'Refresh Time',
        startRequest: 'Start Request',
        stopRequest: 'Stop Request',
        requestCrossComponent: 'Request Cross Component',
        triggerAllRequest: 'Manually Trigger All Automated Requests'
      }
    },
    manage: {
      common: {
        status: {
          enable: 'Enable',
          disable: 'Disable'
        }
      },
      role: {
        title: 'Role List',
        roleName: 'Role Name',
        roleCode: 'Role Code',
        roleStatus: 'Role Status',
        roleDesc: 'Role Description',
        menuAuth: 'Menu Auth',
        buttonAuth: 'Button Auth',
        form: {
          roleName: 'Please enter role name',
          roleCode: 'Please enter role code',
          roleStatus: 'Please select role status',
          roleDesc: 'Please enter role description'
        },
        addRole: 'Add Role',
        editRole: 'Edit Role'
      },
      user: {
        title: 'User List',
        userName: 'User Name',
        userGender: 'Gender',
        nickName: 'Nick Name',
        userPhone: 'Phone Number',
        userEmail: 'Email',
        userStatus: 'User Status',
        userRole: 'User Role',
        form: {
          userName: 'Please enter user name',
          userGender: 'Please select gender',
          nickName: 'Please enter nick name',
          userPhone: 'Please enter phone number',
          userEmail: 'Please enter email',
          userStatus: 'Please select user status',
          userRole: 'Please select user role'
        },
        addUser: 'Add User',
        editUser: 'Edit User',
        gender: {
          male: 'Male',
          female: 'Female'
        }
      },
      menu: {
        home: 'Home',
        title: 'Menu List',
        id: 'ID',
        parentId: 'Parent ID',
        menuType: 'Menu Type',
        menuName: 'Menu Name',
        routeName: 'Route Name',
        routePath: 'Route Path',
        pathParam: 'Path Param',
        layout: 'Layout Component',
        page: 'Page Component',
        i18nKey: 'I18n Key',
        icon: 'Icon',
        localIcon: 'Local Icon',
        iconTypeTitle: 'Icon Type',
        order: 'Order',
        constant: 'Constant',
        keepAlive: 'Keep Alive',
        href: 'Href',
        hideInMenu: 'Hide In Menu',
        activeMenu: 'Active Menu',
        multiTab: 'Multi Tab',
        fixedIndexInTab: 'Fixed Index In Tab',
        query: 'Query Params',
        button: 'Button',
        buttonCode: 'Button Code',
        buttonDesc: 'Button Desc',
        menuStatus: 'Menu Status',
        form: {
          home: 'Please select home',
          menuType: 'Please select menu type',
          menuName: 'Please enter menu name',
          routeName: 'Please enter route name',
          routePath: 'Please enter route path',
          pathParam: 'Please enter path param',
          page: 'Please select page component',
          layout: 'Please select layout component',
          i18nKey: 'Please enter i18n key',
          icon: 'Please enter iconify name',
          localIcon: 'Please enter local icon name',
          order: 'Please enter order',
          keepAlive: 'Please select whether to cache route',
          href: 'Please enter href',
          hideInMenu: 'Please select whether to hide menu',
          activeMenu: 'Please select route name of the highlighted menu',
          multiTab: 'Please select whether to support multiple tabs',
          fixedInTab: 'Please select whether to fix in the tab',
          fixedIndexInTab: 'Please enter the index fixed in the tab',
          queryKey: 'Please enter route parameter Key',
          queryValue: 'Please enter route parameter Value',
          button: 'Please select whether it is a button',
          buttonCode: 'Please enter button code',
          buttonDesc: 'Please enter button description',
          menuStatus: 'Please select menu status'
        },
        addMenu: 'Add Menu',
        editMenu: 'Edit Menu',
        addChildMenu: 'Add Child Menu',
        type: {
          directory: 'Directory',
          menu: 'Menu'
        },
        iconType: {
          iconify: 'Iconify Icon',
          local: 'Local Icon'
        }
      }
    }
  },
  form: {
    required: 'Cannot be empty',
    userName: {
      required: 'Please enter user name',
      invalid: 'User name format is incorrect'
    },
    phone: {
      required: 'Please enter phone number',
      invalid: 'Phone number format is incorrect'
    },
    pwd: {
      required: 'Please enter password',
      invalid: '6-18 characters, including letters, numbers, and underscores'
    },
    confirmPwd: {
      required: 'Please enter password again',
      invalid: 'The two passwords are inconsistent'
    },
    code: {
      required: 'Please enter verification code',
      invalid: 'Verification code format is incorrect'
    },
    email: {
      required: 'Please enter email',
      invalid: 'Email format is incorrect'
    }
  },
  dropdown: {
    closeCurrent: 'Close Current',
    closeOther: 'Close Other',
    closeLeft: 'Close Left',
    closeRight: 'Close Right',
    closeAll: 'Close All'
  },
  icon: {
    themeConfig: 'Theme Configuration',
    themeSchema: 'Theme Schema',
    lang: 'Switch Language',
    fullscreen: 'Fullscreen',
    fullscreenExit: 'Exit Fullscreen',
    reload: 'Reload Page',
    collapse: 'Collapse Menu',
    expand: 'Expand Menu',
    pin: 'Pin',
    unpin: 'Unpin'
  },
  datatable: {
    itemCount: 'Total {total} items'
  },
  vnetPages: {
    common: {
      search: 'Search',
      searchPlaceholder: 'Search...',
      add: 'Add',
      edit: 'Edit',
      delete: 'Delete',
      detail: 'Detail',
      action: 'Action',
      save: 'Save',
      cancel: 'Cancel',
      confirm: 'Confirm',
      status: 'Status',
      active: 'Active',
      inactive: 'Inactive',
      yes: 'Yes',
      no: 'No',
      back: 'Back',
      loading: 'Loading...',
      comingSoon: 'Coming soon',
      success: 'Success',
      error: 'Error',
      warning: 'Warning',
      all: 'All'
    },
    dashboard: {
      revenueToday: 'Revenue Today',
      revenueChart: 'Revenue Chart',
      activeMachines: 'Active Machines',
      machine: 'Machine',
      member: 'Member',
      startTime: 'Start Time',
      duration: 'Duration',
      stats: {
        members: 'Members',
        onlineMachines: 'Online Machines',
        playing: 'Playing',
        revenueToday: "Today's Revenue"
      }
    },
    machines: {
      title: 'Machines',
      searchPlaceholder: 'Search machine code / group',
      add: 'Add Machine',
      edit: 'Edit Machine',
      code: 'Machine Code',
      group: 'Group',
      cpu: 'CPU',
      gpu: 'GPU',
      ram: 'RAM (GB)',
      disk: 'Disk (GB)',
      os: 'Operating System',
      osPlaceholder: 'e.g. Windows 11',
      ip: 'IP',
      lastHeartbeat: 'Last Heartbeat',
      lastSeen: 'Last Seen',
      createdAt: 'Created At',
      updatedAt: 'Updated At',
      detail: 'Detail',
      statusLabels: {
        offline: 'Offline',
        available: 'Available',
        inUse: 'In Use',
        maintenance: 'Maintenance'
      },
      form: {
        code: 'Machine Code',
        group: 'Group',
        cpu: 'CPU',
        gpu: 'GPU',
        ram: 'RAM (GB)',
        disk: 'Disk (GB)',
        os: 'Operating System',
        codeRequired: 'Please enter machine code',
        groupRequired: 'Please select a group'
      },
      messages: {
        addSuccess: 'Machine added successfully',
        editSuccess: 'Machine updated successfully',
        deleteConfirm: 'Delete machine "{code}"?',
        deleteSuccess: 'Deleted successfully',
        deleteError: 'Failed to delete machine',
        loadError: 'Error loading data',
        saveError: 'Error saving'
      },
      tabs: {
        info: 'Info',
        sessions: 'Sessions',
        assets: 'Assets',
        remote: 'Remote'
      },
      remote: {
        shutdown: 'Shutdown',
        restart: 'Restart',
        lock: 'Lock',
        message: 'Message',
        screenshot: 'Screenshot',
        sendNotification: 'Send Notification',
        notificationPlaceholder: 'Enter notification content...',
        send: 'Send',
        confirmAction: 'Execute "{action}" on machine {code}?',
        actionSent: 'Command {action} sent',
        notificationSent: 'Notification sent',
        notificationError: 'Error sending notification'
      },
      asset: {
        name: 'Name',
        type: 'Type',
        serial: 'Serial',
        status: 'Status'
      }
    },
    machineGroups: {
      title: 'Machine Groups',
      name: 'Name',
      color: 'Color',
      colorPlaceholder: 'e.g. #1890ff',
      pricePerHour: 'Price/Hour',
      sortOrder: 'Sort Order',
      description: 'Description',
      create: 'Create Group',
      edit: 'Edit Group',
      nameRequired: 'Please enter group name',
      createSuccess: 'Group created successfully',
      editSuccess: 'Group updated successfully',
      deleteConfirm: 'Delete group "{name}"?',
      deleteSuccess: 'Deleted successfully',
      loadError: 'Error loading data',
      saveError: 'Error saving'
    },
    memberGroups: {
      title: 'Member Groups',
      name: 'Group Name',
      minSpent: 'Min. Spent',
      discountPercent: 'Discount %',
      isDefault: 'Default',
      createdAt: 'Created At',
      create: 'Create Group',
      edit: 'Edit Group',
      nameRequired: 'Please enter group name',
      createSuccess: 'Group created successfully',
      editSuccess: 'Group updated successfully',
      deleteConfirm: 'Delete group "{name}"?',
      deleteSuccess: 'Deleted successfully',
      loadError: 'Error loading data',
      saveError: 'Error saving'
    },
    members: {
      title: 'Members',
      searchPlaceholder: 'Search by name / phone',
      add: 'Add Member',
      edit: 'Edit Member',
      username: 'Username',
      fullName: 'Full Name',
      phone: 'Phone',
      email: 'Email',
      balance: 'Balance',
      bonus: 'Bonus',
      group: 'Group',
      password: 'Password',
      passwordPlaceholder: 'Leave blank to keep current',
      isActive: 'Active',
      createdAt: 'Created At',
      lastLogin: 'Last Login',
      detail: 'Detail',
      topUp: 'Top Up',
      topUpAmount: 'Top Up Amount',
      topUpMethod: 'Method',
      resetPassword: 'Reset PW',
      resetPasswordConfirm: 'Enter new password for this member',
      resetPasswordRequired: 'Password is required',
      cash: 'Cash',
      transfer: 'Transfer',
      eWallet: 'E-Wallet',
      transactions: 'Transactions',
      sessions: 'Sessions',
      amount: 'Amount',
      type: 'Type',
      method: 'Method',
      reference: 'Reference',
      form: {
        username: 'Username',
        fullName: 'Full Name',
        phone: 'Phone Number',
        email: 'Email',
        password: 'Password',
        isActive: 'Active',
        usernameRequired: 'Please enter username',
        passwordRequired: 'Please enter password',
        amountRequired: 'Please enter amount',
        methodRequired: 'Please select method'
      },
      messages: {
        addSuccess: 'Member added successfully',
        editSuccess: 'Member updated successfully',
        deleteConfirm: 'Delete member "{name}"?',
        deleteSuccess: 'Deleted successfully',
        topUpSuccess: 'Top up successful',
        resetPasswordSuccess: 'Password reset successful',
        topUpError: 'Error topping up',
        loadError: 'Error loading data',
        saveError: 'Error saving',
        loadDetailError: 'Error loading member info'
      }
    },
    sessions: {
      title: 'Active Sessions',
      activeSessions: 'Active Sessions',
      totalActive: 'Total: {count} active sessions',
      machineCode: 'Machine',
      member: 'Member',
      startTime: 'Start Time',
      duration: 'Duration',
      endTime: 'End Time',
      status: 'Status',
      running: 'Running',
      ended: 'Ended',
      end: 'End',
      messages: {
        loadError: 'Error loading data',
        endConfirm: 'End session of "{member}" on machine {code}?',
        endSuccess: 'Session ended'
      }
    },
    shifts: {
      title: 'Shifts',
      openShift: 'Open Shift',
      closeShift: 'Close Shift',
      handover: 'Handover',
      startTime: 'Start',
      endTime: 'End',
      openingCash: 'Opening Cash',
      closingCash: 'Closing Cash',
      cashIn: 'Cash In',
      cashOut: 'Cash Out',
      reason: 'Reason',
      amount: 'Amount',
      open: 'Open',
      closed: 'Closed',
      user: 'User',
      form: {
        openingCash: 'Opening Cash',
        closingCash: 'Closing Cash',
        cashIn: 'Cash In',
        cashOut: 'Cash Out',
        reason: 'Reason',
        handoverType: 'Handover Type',
        amountRequired: 'Enter amount',
        amountPlaceholder: 'Enter amount',
        handoverTypePlaceholder: 'Select handover type'
      },
      messages: {
        loadError: 'Error loading data',
        openSuccess: 'Shift opened successfully',
        openError: 'Error opening shift',
        closeSuccess: 'Shift closed successfully',
        closeError: 'Error closing shift',
        handoverSuccess: 'Shift handover successful',
        handoverError: 'Error shifting handover'
      }
    },
    orders: {
      title: 'Orders',
      newOrder: 'New Order',
      newOrderMessage: 'A new order needs processing',
      orderCode: 'Order Code',
      search: 'Search',
      member: 'Member',
      total: 'Total',
      createdAt: 'Created At',
      completedAt: 'Completed At',
      status: 'Status',
      note: 'Note',
      product: 'Product',
      products: 'Products in Order',
      quantity: 'Quantity',
      unitPrice: 'Unit Price',
      subtotal: 'Subtotal',
      createOrder: 'Create Order',
      selectProduct: 'Select Product',
      options: 'Options',
      machineCode: 'Machine',
      operator: 'Operator',
      cancel: 'Cancel',
      confirm: 'Confirm',
      pay: 'Pay',
      orderType: 'Type',
      topup: 'Topup',
      product: 'Product',
      approve: 'Approve',
      reject: 'Reject',
      paymentMethod: 'Payment Method',
      previewTotal: 'Preview Total',
      statusLabels: {
        pending: 'Pending',
        confirmed: 'Confirmed',
        completed: 'Completed',
        cancelled: 'Cancelled'
      },
      detail: 'Detail',
      messages: {
        loadError: 'Error loading data',
        loadDetailError: 'Error loading details',
        updateSuccess: 'Status updated successfully',
        updateError: 'Error updating status',
        paymentConfirm: 'Confirm payment for order {code}?',
        cancelConfirm: 'Confirm cancel order {code}?',
        paymentSuccess: 'Payment successful',
        createSuccess: 'Order created successfully',
        createError: 'Error creating order'
      }
    },
    products: {
      title: 'Products',
      searchPlaceholder: 'Search...',
      add: 'Add Product',
      edit: 'Edit Product',
      name: 'Name',
      category: 'Category',
      price: 'Price',
      description: 'Description',
      image: 'Image',
      imageUrl: 'Image URL',
      supplier: 'Supplier',
      isActive: 'Active',
      hasStock: 'Track Stock',
      stockQuantity: 'Quantity',
      currentStock: 'Stock',
      ingredients: 'Ingredients',
      unit: 'Unit',
      minStock: 'Min Stock',
      pricePerUnit: 'Price/Unit',
      isRetail: 'Retail Item',
      nonRetail: 'Ingredient',
      optionGroups: 'Options',
      optionName: 'Option Name',
      optionIngredientHint: 'Material consumed when this option is selected',
      addOption: 'Add Option',
      optionItem: 'Option Item',
      stockNote: 'Note',
      stockUnitPrice: 'Unit Price',
      stockIn: 'Stock In',
      stockOut: 'Stock Out',
      stockInDescription: 'Stock In product',
      stockOutDescription: 'Stock Out product',
      addIngredient: 'Add ingredient...',
      units: {
        piece: 'Piece',
        box: 'Box',
        bottle: 'Bottle',
        can: 'Can',
        kg: 'Kg',
        liter: 'Liter',
        pack: 'Pack',
        bag: 'Bag',
        crate: 'Crate',
        glass: 'Glass',
        cup: 'Cup',
        plate: 'Plate',
        bowl: 'Bowl',
        portion: 'Portion',
        serving: 'Serving',
        slice: 'Slice',
        tablet: 'Tablet',
        stick: 'Stick',
        rod: 'Rod'
      },
      form: {
        name: 'Name',
        category: 'Category',
        price: 'Price',
        nameRequired: 'Please enter name',
        priceRequired: 'Please enter price',
        categoryPlaceholder: 'Select category',
        uploadImage: 'Upload Image',
        note: 'Note',
        quantityRequired: 'Please enter quantity'
      },
      messages: {
        addSuccess: 'Added successfully',
        editSuccess: 'Updated successfully',
        deleteConfirm: 'Are you sure you want to delete this product?',
        deleteSuccess: 'Deleted successfully',
        loadError: 'Error loading data',
        saveError: 'Error saving data',
        addIngredientSuccess: 'Ingredient added',
        deleteIngredientSuccess: 'Ingredient removed',
        stockInSuccess: 'Stock in successful',
        stockOutSuccess: 'Stock out successful',
        noIngredients: 'No ingredients linked to this product'
      }
    },
    suppliers: {
      title: 'Suppliers',
      name: 'Name',
      phone: 'Phone',
      email: 'Email',
      addSupplier: 'Add Supplier',
      form: {
        nameRequired: 'Please enter name'
      },
      messages: {
        addSuccess: 'Added successfully',
        editSuccess: 'Updated successfully',
        loadError: 'Error loading data',
        saveError: 'Error saving data'
      }
    },
    warehouses: {
      title: 'Warehouses',
      name: 'Name',
      address: 'Address',
      addWarehouse: 'Add Warehouse',
      form: {
        nameRequired: 'Please enter name'
      },
      messages: {
        addSuccess: 'Added successfully',
        editSuccess: 'Updated successfully',
        loadError: 'Error loading data',
        saveError: 'Error saving data'
      }
    },
    stockTransactions: {
      title: 'Stock Transactions',
      createTransaction: 'Create Transaction',
      type: 'Type',
      product: 'Product',
      transactionType: 'Type',
      importLabel: 'Import',
      exportLabel: 'Export',
      quantity: 'Quantity',
      unitPrice: 'Unit Price',
      totalPrice: 'Total Price',
      before: 'Before',
      after: 'After',
      createdAt: 'Created At',
      form: {
        productRequired: 'Select product',
        quantityRequired: 'Enter quantity'
      },
      messages: {
        createTransactionSuccess: 'Transaction created successfully',
        loadError: 'Error loading data',
        saveError: 'Error saving data'
      }
    },
    reports: {
      title: 'Reports',
      from: 'From',
      to: 'To',
      dailyRevenue: 'Daily Revenue',
      date: 'Date',
      totalRevenue: 'Total Revenue',
      orderCount: 'Orders',
      memberCount: 'Members',
      monthlyRevenue: 'Monthly Revenue',
      month: 'Month',
      byMember: 'By Member',
      memberCode: 'Member Code',
      totalSpent: 'Total Spent',
      visitCount: 'Visits',
      byMachine: 'By Machine',
      machineName: 'Machine',
      revenue: 'Revenue',
      hours: 'Hours',
      sessionCount: 'Sessions',
      member: 'Member',
      machine: 'Machine',
      messages: {
        loadError: 'Error loading report'
      }
    },
    bookings: {
      title: 'Bookings',
      customer: 'Customer',
      machineCode: 'Machine',
      from: 'From',
      to: 'To',
      deposit: 'Deposit',
      checkIn: 'Check In',
      statusLabels: {
        pending: 'Pending',
        confirmed: 'Confirmed',
        checkedIn: 'Checked In',
        cancelled: 'Cancelled',
        noShow: 'No Show'
      },
      messages: {
        loadError: 'Error loading data',
        checkInConfirm: 'Check in "{customer}" at machine {code}?',
        checkInSuccess: 'Check in successful',
        cancelConfirm: 'Cancel booking of "{customer}"?',
        cancelSuccess: 'Booking cancelled'
      }
    },
    categories: {
      title: 'Categories',
      name: 'Name',
      icon: 'Icon',
      order: 'Order',
      parent: 'Parent Category',
      searchPlaceholder: 'Search categories...',
      add: 'Add Category',
      edit: 'Edit Category',
      iconPlaceholder: 'Select an icon',
      parentPlaceholder: 'None (top level)',
      form: {
        nameRequired: 'Please enter category name'
      },
      messages: {
        addSuccess: 'Category added successfully',
        editSuccess: 'Category updated successfully',
        saveError: 'Error saving category',
        deleteConfirm: 'Delete this category?',
        deleteSuccess: 'Category deleted successfully'
      }
    },
    promotions: {
      title: 'Promotions',
      searchPlaceholder: 'Search promotions...',
      add: 'Add Promotion',
      edit: 'Edit Promotion',
      name: 'Name',
      type: 'Type',
      priority: 'Priority',
      isActive: 'Active',
      validPeriod: 'Valid Period',
      conditions: 'Conditions',
      rewards: 'Rewards',
      luckySpinRewards: 'Lucky Spin Rewards',
      value: 'Value',
      probability: 'Probability',
      form: {
        name: 'Name',
        type: 'Type',
        typePlaceholder: 'e.g. percentage, fixed, combo',
        nameRequired: 'Please enter name',
        typeRequired: 'Please enter type'
      },
      messages: {
        addSuccess: 'Promotion added successfully',
        editSuccess: 'Promotion updated successfully',
        deleteConfirm: 'Delete promotion "{name}"?',
        deleteSuccess: 'Deleted successfully',
        loadError: 'Error loading data',
        loadRewardsError: 'Error loading rewards',
        saveError: 'Error saving'
      }
    },
    chat: {
      title: 'Chat',
      rooms: 'Rooms',
      with: 'with',
      newRoom: 'New',
      noMessages: 'No messages yet',
      selectRoom: 'Select a room',
      inputPlaceholder: 'Type a message...',
      send: 'Send',
      createRoom: 'New Room',
      recipients: 'Recipients',
      selectRecipients: 'Select recipients',
      messages: {
        selectAtLeastOne: 'Select at least 1 person',
        loadRoomsError: 'Error loading rooms',
        loadMessagesError: 'Error loading messages',
        sendError: 'Error sending message',
        createSuccess: 'Room created successfully',
        createError: 'Error creating room'
      },
      newMessage: 'New message'
    },
    settings: {
      title: 'Settings',
      general: 'General',
      billing: 'Billing',
      printing: 'Printing',
      invoice: 'Invoice',
      address: 'Address',
      phone: 'Phone Number',
      email: 'Email',
      timezone: 'Timezone',
      limits: 'Limits',
      maxBookingsPerDay: 'Max Bookings / Day',
      maxBookingsPerMember: 'Max Bookings / Member',
      cancelBeforeMinutes: 'Cancel Before (minutes)',
      maxDebt: 'Max Debt (VND)',
      printerType: 'Printer Type',
      thermal: 'Thermal',
      laser: 'Laser',
      inkjet: 'Inkjet',
      printerName: 'Printer Name',
      paperSize: 'Paper Size',
      paperSizePlaceholder: 'e.g. 80mm',
      autoPrint: 'Auto Print',
      invoiceTitle: 'Invoice Title',
      invoiceFooter: 'Invoice Footer',
      taxCode: 'Tax Code',
      invoiceStartNumber: 'Starting Invoice Number',
      showLogo: 'Show Logo',
      messages: {
        saveSuccess: 'Settings saved successfully',
        saveError: 'Error saving settings'
      }
    },
    audit: {
      title: 'Audit Logs',
      action: 'Action',
      target: 'Target',
      from: 'From',
      to: 'To',
      id: 'ID',
      user: 'User',
      description: 'Description',
      time: 'Time',
      actionLabels: {
        create: 'Create',
        update: 'Update',
        delete: 'Delete',
        topup: 'Top Up',
        refund: 'Refund',
        purchase: 'Purchase',
        pay: 'Payment',
        cancel: 'Cancel'
      },
      entityLabels: {
        member: 'member',
        machine: 'machine',
        order: 'order',
        product: 'product',
        category: 'category',
        combo: 'combo',
        promotion: 'promotion',
        booking: 'booking',
        user: 'user',
        role: 'role'
      },
      messages: {
        loadError: 'Error loading logs'
      }
    },
    combos: {
      title: 'Combos',
      searchPlaceholder: 'Search combos...',
      add: 'Add Combo',
      edit: 'Edit Combo',
      name: 'Combo Name',
      type: 'Type',
      fixedSlot: 'Fixed Slot',
      prepaid: 'Prepaid',
      price: 'Price',
      isActive: 'Active',
      from: 'From',
      to: 'To',
      addSlot: '+ Add Slot',
      minutes: 'Minutes',
      form: {
        name: 'Combo Name',
        type: 'Combo Type',
        price: 'Price',
        minutes: 'Minutes',
        nameRequired: 'Please enter combo name',
        typeRequired: 'Please select type',
        priceRequired: 'Please enter price',
        minutesRequired: 'Please enter minutes'
      },
      messages: {
        addSuccess: 'Combo added successfully',
        editSuccess: 'Combo updated successfully',
        deleteConfirm: 'Delete combo "{name}"?',
        deleteSuccess: 'Deleted successfully',
        loadError: 'Error loading data',
        saveError: 'Error saving'
      }
    },
    backups: {
      title: 'Backup & Restore',
      createBackup: 'Create Backup',
      fileName: 'File Name',
      size: 'Size',
      status: 'Status',
      createdAt: 'Created At',
      completed: 'Completed',
      running: 'Running',
      failed: 'Failed',
      restore: 'Restore',
      messages: {
        loadError: 'Error loading backup list',
        createConfirm: 'Create a new backup? This may take a few minutes.',
        creating: 'Creating backup...',
        restoreConfirm: 'Restore data from backup "{file}"? Current data will be replaced.',
        restoring: 'Restoring data...',
        deleteComingSoon: 'Delete backup feature is under development'
      }
    },
    transactions: {
      title: 'Transaction History',
      from: 'From',
      to: 'To',
      typePlaceholder: 'Transaction type',
      all: 'All',
      searchPlaceholder: 'Search by name / phone / username',
      date: 'Date',
      member: 'Member',
      type: 'Type',
      amount: 'Amount',
      balanceBefore: 'Balance Before',
      balanceAfter: 'Balance After',
      paymentMethod: 'Method',
      description: 'Description',
      createdBy: 'Created By',
      topup: 'Top-up',
      sessionFee: 'Session Fee',
      refund: 'Refund',
      cancel: 'Cancel',
      comboPurchase: 'Combo Purchase',
      messages: {
        loadError: 'Failed to load transactions'
      }
    }
  }
};

export default local;
