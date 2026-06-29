// @ts-nocheck
const local: App.I18n.Schema = {
  system: {
    title: 'VNET Quản Lý',
    updateTitle: 'Thông báo cập nhật phiên bản hệ thống',
    updateContent: 'Đã phát hiện phiên bản mới, bạn có muốn làm mới trang ngay lập tức?',
    updateConfirm: 'Làm mới ngay',
    updateCancel: 'Để sau'
  },
  common: {
    action: 'Thao tác',
    add: 'Thêm',
    addSuccess: 'Thêm thành công',
    backToHome: 'Về trang chủ',
    batchDelete: 'Xóa hàng loạt',
    cancel: 'Hủy',
    close: 'Đóng',
    check: 'Chọn',
    expandColumn: 'Mở rộng cột',
    columnSetting: 'Cài đặt cột',
    config: 'Cấu hình',
    confirm: 'Xác nhận',
    delete: 'Xóa',
    deleteSuccess: 'Xóa thành công',
    confirmDelete: 'Bạn có chắc chắn muốn xóa?',
    edit: 'Sửa',
    warning: 'Cảnh báo',
    error: 'Lỗi',
    index: 'STT',
    keywordSearch: 'Nhập từ khóa tìm kiếm',
    logout: 'Đăng xuất',
    logoutConfirm: 'Bạn có chắc chắn muốn đăng xuất?',
    lookForward: 'Sắp ra mắt',
    modify: 'Chỉnh sửa',
    modifySuccess: 'Chỉnh sửa thành công',
    noData: 'Không có dữ liệu',
    operate: 'Thao tác',
    pleaseCheckValue: 'Vui lòng kiểm tra giá trị hợp lệ',
    refresh: 'Làm mới',
    reset: 'Đặt lại',
    search: 'Tìm kiếm',
    switch: 'Chuyển đổi',
    tip: 'Gợi ý',
    trigger: 'Kích hoạt',
    update: 'Cập nhật',
    updateSuccess: 'Cập nhật thành công',
    userCenter: 'Trung tâm người dùng',
    yesOrNo: {
      yes: 'Có',
      no: 'Không'
    }
  },
  request: {
    logout: 'Đăng xuất sau khi yêu cầu thất bại',
    logoutMsg: 'Trạng thái người dùng không hợp lệ, vui lòng đăng nhập lại',
    logoutWithModal: 'Hiện modal sau khi yêu cầu thất bại và đăng xuất',
    logoutWithModalMsg: 'Trạng thái người dùng không hợp lệ, vui lòng đăng nhập lại',
    refreshToken: 'Token đã hết hạn, làm mới token',
    tokenExpired: 'Token đã hết hạn'
  },
  theme: {
    themeSchema: {
      title: 'Chế độ giao diện',
      light: 'Sáng',
      dark: 'Tối',
      auto: 'Theo hệ thống'
    },
    grayscale: 'Thang độ xám',
    colourWeakness: 'Mù màu',
    layoutMode: {
      title: 'Chế độ bố cục',
      vertical: 'Menu dọc',
      horizontal: 'Menu ngang',
      'vertical-mix': 'Menu dọc hỗn hợp',
      'horizontal-mix': 'Menu ngang hỗn hợp',
      reverseHorizontalMix: 'Đảo vị trí menu cấp 1 và menu con'
    },
    recommendColor: 'Áp dụng thuật toán màu đề xuất',
    recommendColorDesc: 'Thuật toán màu đề xuất tham khảo',
    themeColor: {
      title: 'Màu chủ đề',
      primary: 'Chính',
      info: 'Thông tin',
      success: 'Thành công',
      warning: 'Cảnh báo',
      error: 'Lỗi',
      followPrimary: 'Theo màu chính'
    },
    scrollMode: {
      title: 'Chế độ cuộn',
      wrapper: 'Cuộn ngoài',
      content: 'Cuộn nội dung'
    },
    page: {
      animate: 'Hiệu ứng chuyển trang',
      mode: {
        title: 'Kiểu hiệu ứng chuyển trang',
        fade: 'Mờ dần',
        'fade-slide': 'Trượt',
        'fade-bottom': 'Thu nhỏ',
        'fade-scale': 'Thu phóng',
        'zoom-fade': 'Phóng to mờ',
        'zoom-out': 'Hiện ra',
        none: 'Không'
      }
    },
    fixedHeaderAndTab: 'Cố định header và tab',
    header: {
      height: 'Chiều cao header',
      breadcrumb: {
        visible: 'Hiển thị breadcrumb',
        showIcon: 'Hiển thị icon breadcrumb'
      },
      multilingual: {
        visible: 'Hiển thị nút đa ngôn ngữ'
      },
      globalSearch: {
        visible: 'Hiển thị nút tìm kiếm toàn cục'
      }
    },
    tab: {
      visible: 'Hiển thị thanh tab',
      cache: 'Lưu cache thanh tab',
      height: 'Chiều cao tab',
      mode: {
        title: 'Kiểu tab',
        chrome: 'Chrome',
        button: 'Nút'
      }
    },
    sider: {
      inverted: 'Sidebar tối',
      width: 'Chiều rộng sidebar',
      collapsedWidth: 'Chiều rộng sidebar thu gọn',
      mixWidth: 'Chiều rộng sidebar hỗn hợp',
      mixCollapsedWidth: 'Chiều rộng sidebar hỗn hợp thu gọn',
      mixChildMenuWidth: 'Chiều rộng menu con hỗn hợp'
    },
    footer: {
      visible: 'Hiển thị footer',
      fixed: 'Cố định footer',
      height: 'Chiều cao footer',
      right: 'Footer bên phải'
    },
    watermark: {
      visible: 'Hiển thị watermark toàn màn hình',
      text: 'Nội dung watermark',
      enableUserName: 'Hiển thị tên người dùng trong watermark'
    },
    themeDrawerTitle: 'Cấu hình giao diện',
    pageFunTitle: 'Chức năng trang',
    configOperation: {
      copyConfig: 'Sao chép cấu hình',
      copySuccessMsg: 'Sao chép thành công, vui lòng thay thế biến "themeSettings" trong "src/theme/settings.ts"',
      resetConfig: 'Đặt lại cấu hình',
      resetSuccessMsg: 'Đặt lại thành công'
    }
  },
  route: {
    login: 'Đăng nhập',
    403: 'Không có quyền',
    404: 'Không tìm thấy trang',
    500: 'Lỗi máy chủ',
    'iframe-page': 'Trang iframe',
    home: 'Trang chủ',
    document: 'Tài liệu',
    document_project: 'Tài liệu dự án',
    'document_project-link': 'Tài liệu dự án (Liên kết ngoài)',
    document_vue: 'Tài liệu Vue',
    document_vite: 'Tài liệu Vite',
    document_unocss: 'Tài liệu UnoCSS',
    document_naive: 'Tài liệu Naive UI',
    document_antd: 'Tài liệu Ant Design Vue',
    'document_element-plus': 'Tài liệu Element Plus',
    document_alova: 'Tài liệu Alova',
    'user-center': 'Trung tâm người dùng',
    about: 'Giới thiệu',
    function: 'Chức năng hệ thống',
    alova: 'Ví dụ Alova',
    alova_request: 'Yêu cầu Alova',
    alova_user: 'Danh sách người dùng',
    alova_scenes: 'Yêu cầu theo kịch bản',
    function_tab: 'Tab',
    'function_multi-tab': 'Nhiều tab',
    'function_hide-child': 'Ẩn menu con',
    'function_hide-child_one': 'Ẩn menu con',
    'function_hide-child_two': 'Hai',
    'function_hide-child_three': 'Ba',
    function_request: 'Yêu cầu',
    'function_toggle-auth': 'Chuyển đổi quyền',
    'function_super-page': 'Admin cấp cao thấy',
    system: 'Quản lý hệ thống',
    system_user: 'Quản lý người dùng',
    'system_user-detail': 'Chi tiết người dùng',
    system_role: 'Quản lý vai trò',
    system_menu: 'Quản lý menu',
    'multi-menu': 'Đa cấp menu',
    'multi-menu_first': 'Menu một',
    'multi-menu_first_child': 'Menu một con',
    'multi-menu_second': 'Menu hai',
    'multi-menu_second_child': 'Menu hai con',
    'multi-menu_second_child_home': 'Menu hai con trang chủ',
    exception: 'Trang ngoại lệ',
    exception_403: '403',
    exception_404: '404',
    exception_500: '500',
    plugin: 'Plugin',
    plugin_copy: 'Sao chép',
    plugin_charts: 'Biểu đồ',
    plugin_charts_echarts: 'ECharts',
    plugin_charts_antv: 'AntV',
    plugin_charts_vchart: 'VChart',
    plugin_editor: 'Trình soạn thảo',
    plugin_editor_quill: 'Quill',
    plugin_editor_markdown: 'Markdown',
    plugin_icon: 'Biểu tượng',
    plugin_map: 'Bản đồ',
    plugin_print: 'In ấn',
    plugin_swiper: 'Swiper',
    plugin_video: 'Video',
    plugin_barcode: 'Mã vạch',
    plugin_pinyin: 'pinyin',
    plugin_excel: 'Excel',
    plugin_pdf: 'Xem PDF',
    plugin_gantt: 'Biểu đồ Gantt',
    plugin_gantt_dhtmlx: 'dhtmlxGantt',
    plugin_gantt_vtable: 'VTableGantt',
    plugin_typeit: 'Typeit',
    plugin_tables: 'Bảng',
    plugin_tables_vtable: 'VTable',
    vnet: 'VNET',
    vnet_dashboard: 'Bảng điều khiển',
    vnet_members: 'Hội viên',
    'vnet_members-detail': 'Chi tiết hội viên',
    vnet_machines: 'Máy',
    'vnet_machine-groups': 'Nhóm máy',
    'vnet_member-groups': 'Nhóm hội viên',
    vnet_sessions: 'Phiên',
    vnet_combos: 'Gói dịch vụ',
    vnet_bookings: 'Đặt chỗ',
    vnet_promotions: 'Khuyến mãi',
    vnet_categories: 'Danh mục',
    vnet_products: 'Sản phẩm',
    vnet_orders: 'Đơn hàng',
    vnet_suppliers: 'Nhà cung cấp',
    vnet_warehouses: 'Kho',
    'vnet_stock-transactions': 'Tồn kho',
    vnet_shifts: 'Ca làm việc',
    vnet_reports: 'Báo cáo',
    vnet_transactions: 'Giao dịch',
    vnet_settings: 'Cài đặt',
    vnet_audit: 'Nhật ký hoạt động',
    vnet_stores: 'Cửa hàng',
    vnet_backups: 'Sao lưu',
    vnet_management: 'Quản lý',
    vnet_business: 'Kinh doanh',
    vnet_operations: 'Vận hành',
    vnet_system: 'Hệ thống'
  },
  page: {
    login: {
      common: {
        loginOrRegister: 'Đăng nhập / Đăng ký',
        userNamePlaceholder: 'Vui lòng nhập tên người dùng',
        phonePlaceholder: 'Vui lòng nhập số điện thoại',
        codePlaceholder: 'Vui lòng nhập mã xác nhận',
        passwordPlaceholder: 'Vui lòng nhập mật khẩu',
        confirmPasswordPlaceholder: 'Vui lòng nhập lại mật khẩu',
        codeLogin: 'Đăng nhập mã xác nhận',
        confirm: 'Xác nhận',
        back: 'Quay lại',
        validateSuccess: 'Xác thực thành công',
        loginSuccess: 'Đăng nhập thành công',
        welcomeBack: 'Chào mừng trở lại, {userName}!'
      },
      pwdLogin: {
        title: 'Đăng nhập mật khẩu',
        rememberMe: 'Ghi nhớ tôi',
        forgetPassword: 'Quên mật khẩu?',
        register: 'Đăng ký',
        otherAccountLogin: 'Đăng nhập tài khoản khác',
        otherLoginMode: 'Phương thức đăng nhập khác',
        superAdmin: 'Super Admin',
        admin: 'Admin',
        user: 'Người dùng'
      },
      codeLogin: {
        title: 'Đăng nhập mã xác nhận',
        getCode: 'Lấy mã xác nhận',
        reGetCode: 'Lấy lại sau {time}s',
        sendCodeSuccess: 'Gửi mã xác nhận thành công',
        imageCodePlaceholder: 'Vui lòng nhập mã xác nhận hình ảnh'
      },
      register: {
        title: 'Đăng ký',
        agreement: 'Tôi đã đọc và đồng ý với',
        protocol: '《Thỏa thuận người dùng》',
        policy: '《Chính sách bảo mật》'
      },
      resetPwd: {
        title: 'Đặt lại mật khẩu'
      },
      bindWeChat: {
        title: 'Liên kết WeChat'
      }
    },
    about: {
      title: 'Giới thiệu',
      introduction: `SoybeanAdmin là một mẫu quản trị backend mạnh mẽ và thanh lịch, dựa trên công nghệ frontend mới nhất bao gồm Vue3, Vite5, TypeScript, Pinia và UnoCSS. Nó tích hợp cấu hình giao diện phong phú và các component, code chuẩn mực, hệ thống route file tự động. Ngoài ra, nó còn sử dụng giải pháp dữ liệu mock trực tuyến dựa trên ApiFox. SoybeanAdmin cung cấp giải pháp quản trị backend tất cả trong một, không cần cấu hình thêm, sẵn sàng sử dụng. Đây cũng là một thực hành tốt nhất để học nhanh các công nghệ tiên tiến.`,
      projectInfo: {
        title: 'Thông tin dự án',
        version: 'Phiên bản',
        latestBuildTime: 'Thời gian build mới nhất',
        githubLink: 'Liên kết Github',
        previewLink: 'Liên kết xem trước'
      },
      prdDep: 'Phụ thuộc Production',
      devDep: 'Phụ thuộc Development'
    },
    home: {
      branchDesc:
        'Để thuận tiện cho việc phát triển và cập nhật merge, chúng tôi đã tinh gọn code của nhánh main, chỉ giữ lại menu trang chủ, phần còn lại đã được chuyển sang nhánh example để bảo trì. Địa chỉ xem trước hiển thị nội dung của nhánh example.',
      greeting: 'Chào buổi sáng, {userName}, hôm nay lại là một ngày tràn đầy năng lượng!',
      weatherDesc: 'Hôm nay trời nhiều mây đến quang, 20°C - 25°C!',
      projectCount: 'Số dự án',
      todo: 'Việc cần làm',
      message: 'Tin nhắn',
      downloadCount: 'Lượt tải',
      registerCount: 'Lượt đăng ký',
      schedule: 'Lịch làm việc',
      study: 'Học tập',
      work: 'Làm việc',
      rest: 'Nghỉ ngơi',
      entertainment: 'Giải trí',
      visitCount: 'Lượt truy cập',
      turnover: 'Doanh thu',
      dealCount: 'Số giao dịch',
      projectNews: {
        title: 'Tin tức dự án',
        moreNews: 'Thêm tin tức',
        desc1: 'Soybean đã tạo dự án mã nguồn mở soybean-admin vào ngày 28 tháng 5 năm 2021!',
        desc2: 'Yanbowe đã gửi một lỗi tới soybean-admin, thanh multi-tab không tự động thích ứng.',
        desc3: 'Soybean đã sẵn sàng chuẩn bị đầy đủ cho việc phát hành soybean-admin!',
        desc4: 'Soybean đang bận viết tài liệu dự án cho soybean-admin!',
        desc5: 'Soybean vừa viết một số trang workbench, tạm thời đủ dùng!'
      },
      creativity: 'Sáng tạo'
    },
    function: {
      tab: {
        tabOperate: {
          title: 'Thao tác tab',
          addTab: 'Thêm tab',
          addTabDesc: 'Đến trang giới thiệu',
          closeTab: 'Đóng tab',
          closeCurrentTab: 'Đóng tab hiện tại',
          closeAboutTab: 'Đóng tab "Giới thiệu"',
          addMultiTab: 'Thêm nhiều tab',
          addMultiTabDesc1: 'Đến trang nhiều tab',
          addMultiTabDesc2: 'Đến trang nhiều tab (có tham số query)'
        },
        tabTitle: {
          title: 'Tiêu đề tab',
          changeTitle: 'Đổi tiêu đề',
          change: 'Đổi',
          resetTitle: 'Đặt lại tiêu đề',
          reset: 'Đặt lại'
        }
      },
      multiTab: {
        routeParam: 'Tham số route',
        backTab: 'Quay lại function_tab'
      },
      toggleAuth: {
        toggleAccount: 'Chuyển tài khoản',
        authHook: 'Hàm hook quyền `hasAuth`',
        superAdminVisible: 'Super Admin thấy',
        adminVisible: 'Admin thấy',
        adminOrUserVisible: 'Admin và Người dùng thấy'
      },
      request: {
        repeatedErrorOccurOnce: 'Lỗi yêu cầu lặp lại chỉ xảy ra một lần',
        repeatedError: 'Lỗi yêu cầu lặp lại',
        repeatedErrorMsg1: 'Lỗi yêu cầu tùy chỉnh 1',
        repeatedErrorMsg2: 'Lỗi yêu cầu tùy chỉnh 2'
      }
    },
    alova: {
      scenes: {
        captchaSend: 'Gửi captcha',
        autoRequest: 'Tự động yêu cầu',
        visibilityRequestTips: 'Tự động yêu cầu khi chuyển cửa sổ trình duyệt',
        pollingRequestTips: 'Sẽ yêu cầu mỗi 3 giây',
        networkRequestTips: 'Tự động yêu cầu sau khi kết nối mạng lại',
        refreshTime: 'Thời gian làm mới',
        startRequest: 'Bắt đầu yêu cầu',
        stopRequest: 'Dừng yêu cầu',
        requestCrossComponent: 'Yêu cầu qua component',
        triggerAllRequest: 'Kích hoạt tất cả yêu cầu tự động'
      }
    },
    manage: {
      common: {
        status: {
          enable: 'Bật',
          disable: 'Tắt'
        }
      },
      role: {
        title: 'Danh sách vai trò',
        roleName: 'Tên vai trò',
        roleCode: 'Mã vai trò',
        roleStatus: 'Trạng thái vai trò',
        roleDesc: 'Mô tả vai trò',
        menuAuth: 'Quyền menu',
        buttonAuth: 'Quyền nút',
        form: {
          roleName: 'Vui lòng nhập tên vai trò',
          roleCode: 'Vui lòng nhập mã vai trò',
          roleStatus: 'Vui lòng chọn trạng thái vai trò',
          roleDesc: 'Vui lòng nhập mô tả vai trò'
        },
        addRole: 'Thêm vai trò',
        editRole: 'Sửa vai trò'
      },
      user: {
        title: 'Danh sách người dùng',
        userName: 'Tên người dùng',
        userGender: 'Giới tính',
        nickName: 'Biệt danh',
        userPhone: 'Số điện thoại',
        userEmail: 'Email',
        userStatus: 'Trạng thái người dùng',
        userRole: 'Vai trò người dùng',
        form: {
          userName: 'Vui lòng nhập tên người dùng',
          userGender: 'Vui lòng chọn giới tính',
          nickName: 'Vui lòng nhập biệt danh',
          userPhone: 'Vui lòng nhập số điện thoại',
          userEmail: 'Vui lòng nhập email',
          userStatus: 'Vui lòng chọn trạng thái người dùng',
          userRole: 'Vui lòng chọn vai trò người dùng'
        },
        addUser: 'Thêm người dùng',
        editUser: 'Sửa người dùng',
        gender: {
          male: 'Nam',
          female: 'Nữ'
        }
      },
      menu: {
        home: 'Trang chủ',
        title: 'Danh sách menu',
        id: 'ID',
        parentId: 'ID menu cha',
        menuType: 'Loại menu',
        menuName: 'Tên menu',
        routeName: 'Tên route',
        routePath: 'Đường dẫn route',
        pathParam: 'Tham số đường dẫn',
        layout: 'Component layout',
        page: 'Component trang',
        i18nKey: 'Khóa i18n',
        icon: 'Biểu tượng',
        localIcon: 'Biểu tượng cục bộ',
        iconTypeTitle: 'Loại biểu tượng',
        order: 'Thứ tự',
        constant: 'Route hằng số',
        keepAlive: 'Giữ cache',
        href: 'Liên kết ngoài',
        hideInMenu: 'Ẩn trong menu',
        activeMenu: 'Menu đang active',
        multiTab: 'Hỗ trợ nhiều tab',
        fixedIndexInTab: 'Chỉ số cố định trong tab',
        query: 'Tham số query',
        button: 'Nút',
        buttonCode: 'Mã nút',
        buttonDesc: 'Mô tả nút',
        menuStatus: 'Trạng thái menu',
        form: {
          home: 'Vui lòng chọn trang chủ',
          menuType: 'Vui lòng chọn loại menu',
          menuName: 'Vui lòng nhập tên menu',
          routeName: 'Vui lòng nhập tên route',
          routePath: 'Vui lòng nhập đường dẫn route',
          pathParam: 'Vui lòng nhập tham số đường dẫn',
          page: 'Vui lòng chọn component trang',
          layout: 'Vui lòng chọn component layout',
          i18nKey: 'Vui lòng nhập khóa i18n',
          icon: 'Vui lòng nhập tên iconify',
          localIcon: 'Vui lòng chọn biểu tượng cục bộ',
          order: 'Vui lòng nhập thứ tự',
          keepAlive: 'Vui lòng chọn có cache route không',
          href: 'Vui lòng nhập liên kết ngoài',
          hideInMenu: 'Vui lòng chọn có ẩn menu không',
          activeMenu: 'Vui lòng chọn tên route của menu được highlight',
          multiTab: 'Vui lòng chọn có hỗ trợ nhiều tab không',
          fixedInTab: 'Vui lòng chọn có cố định trong tab không',
          fixedIndexInTab: 'Vui lòng nhập chỉ số cố định trong tab',
          queryKey: 'Vui lòng nhập Key tham số route',
          queryValue: 'Vui lòng nhập Value tham số route',
          button: 'Vui lòng chọn có phải nút không',
          buttonCode: 'Vui lòng nhập mã nút',
          buttonDesc: 'Vui lòng nhập mô tả nút',
          menuStatus: 'Vui lòng chọn trạng thái menu'
        },
        addMenu: 'Thêm menu',
        editMenu: 'Sửa menu',
        addChildMenu: 'Thêm menu con',
        type: {
          directory: 'Thư mục',
          menu: 'Menu'
        },
        iconType: {
          iconify: 'Iconify',
          local: 'Cục bộ'
        }
      }
    }
  },
  form: {
    required: 'Không được để trống',
    userName: {
      required: 'Vui lòng nhập tên người dùng',
      invalid: 'Định dạng tên người dùng không hợp lệ'
    },
    phone: {
      required: 'Vui lòng nhập số điện thoại',
      invalid: 'Định dạng số điện thoại không hợp lệ'
    },
    pwd: {
      required: 'Vui lòng nhập mật khẩu',
      invalid: '6-18 ký tự, bao gồm chữ cái, số và dấu gạch dưới'
    },
    confirmPwd: {
      required: 'Vui lòng nhập lại mật khẩu',
      invalid: 'Hai mật khẩu không khớp'
    },
    code: {
      required: 'Vui lòng nhập mã xác nhận',
      invalid: 'Định dạng mã xác nhận không hợp lệ'
    },
    email: {
      required: 'Vui lòng nhập email',
      invalid: 'Định dạng email không hợp lệ'
    }
  },
  dropdown: {
    closeCurrent: 'Đóng hiện tại',
    closeOther: 'Đóng các tab khác',
    closeLeft: 'Đóng bên trái',
    closeRight: 'Đóng bên phải',
    closeAll: 'Đóng tất cả'
  },
  icon: {
    themeConfig: 'Cấu hình giao diện',
    themeSchema: 'Chế độ giao diện',
    lang: 'Chuyển đổi ngôn ngữ',
    fullscreen: 'Toàn màn hình',
    fullscreenExit: 'Thoát toàn màn hình',
    reload: 'Làm mới trang',
    collapse: 'Thu gọn menu',
    expand: 'Mở rộng menu',
    pin: 'Ghim',
    unpin: 'Bỏ ghim'
  },
  datatable: {
    itemCount: 'Tổng cộng {total} mục'
  },
  vnetPages: {
    common: {
      search: 'Tìm kiếm',
      searchPlaceholder: 'Tìm kiếm...',
      add: 'Thêm',
      edit: 'Sửa',
      delete: 'Xóa',
      detail: 'Chi tiết',
      action: 'Thao tác',
      save: 'Lưu',
      cancel: 'Hủy',
      confirm: 'Xác nhận',
      status: 'Trạng thái',
      active: 'Kích hoạt',
      inactive: 'Không kích hoạt',
      yes: 'Có',
      no: 'Không',
      back: 'Quay lại',
      loading: 'Đang tải...',
      comingSoon: 'Tính năng đang phát triển',
      success: 'Thành công',
      error: 'Lỗi',
      warning: 'Cảnh báo',
      all: 'Tất cả'
    },
    dashboard: {
      revenueToday: 'Doanh thu hôm nay',
      revenueChart: 'Biểu đồ doanh thu',
      activeMachines: 'Máy đang hoạt động',
      machine: 'Máy',
      member: 'Hội viên',
      startTime: 'Bắt đầu',
      duration: 'Thời gian',
      stats: {
        members: 'Hội viên',
        onlineMachines: 'Máy online',
        playing: 'Đang chơi',
        revenueToday: 'Doanh thu hôm nay'
      }
    },
    machines: {
      title: 'Máy',
      searchPlaceholder: 'Tìm mã máy / nhóm',
      add: 'Thêm máy',
      edit: 'Sửa máy',
      code: 'Mã máy',
      group: 'Nhóm',
      cpu: 'CPU',
      gpu: 'GPU',
      ram: 'RAM (GB)',
      disk: 'Ổ cứng (GB)',
      os: 'Hệ điều hành',
      osPlaceholder: 'VD: Windows 11',
      ip: 'IP',
      lastHeartbeat: 'Lần cuối',
      lastSeen: 'Lần cuối heartbeat',
      createdAt: 'Ngày tạo',
      updatedAt: 'Ngày cập nhật',
      detail: 'Chi tiết',
      statusLabels: {
        offline: 'Ngoại tuyến',
        available: 'Sẵn sàng',
        inUse: 'Đang sử dụng',
        maintenance: 'Bảo trì'
      },
      form: {
        code: 'Mã máy',
        group: 'Nhóm',
        cpu: 'CPU',
        gpu: 'GPU',
        ram: 'RAM (GB)',
        disk: 'Ổ cứng (GB)',
        os: 'Hệ điều hành',
        codeRequired: 'Vui lòng nhập mã máy',
        groupRequired: 'Vui lòng chọn nhóm'
      },
      messages: {
        addSuccess: 'Thêm máy thành công',
        editSuccess: 'Cập nhật máy thành công',
        deleteConfirm: 'Xóa máy "{code}"?',
        deleteSuccess: 'Xóa thành công',
        deleteError: 'Lỗi xóa máy',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi khi lưu'
      },
      tabs: {
        info: 'Thông tin',
        sessions: 'Phiên',
        assets: 'Tài sản',
        remote: 'Điều khiển'
      },
      remote: {
        shutdown: 'Tắt máy',
        restart: 'Khởi động lại',
        lock: 'Khóa',
        message: 'Tin nhắn',
        screenshot: 'Chụp màn hình',
        sendNotification: 'Gửi thông báo',
        notificationPlaceholder: 'Nhập nội dung thông báo...',
        send: 'Gửi',
        confirmAction: 'Thực hiện "{action}" trên máy {code}?',
        actionSent: 'Đã gửi lệnh {action}',
        notificationSent: 'Đã gửi thông báo',
        notificationError: 'Lỗi gửi thông báo'
      },
      asset: {
        name: 'Tên',
        type: 'Loại',
        serial: 'Serial',
        status: 'Trạng thái'
      }
    },
    machineGroups: {
      title: 'Nhóm máy',
      name: 'Tên nhóm',
      color: 'Màu sắc',
      colorPlaceholder: 'VD: #1890ff',
      sortOrder: 'Thứ tự',
      description: 'Mô tả',
      create: 'Tạo nhóm',
      edit: 'Sửa nhóm',
      nameRequired: 'Vui lòng nhập tên nhóm',
      createSuccess: 'Tạo nhóm thành công',
      editSuccess: 'Cập nhật nhóm thành công',
      deleteConfirm: 'Xóa nhóm "{name}"?',
      deleteSuccess: 'Xóa thành công',
      loadError: 'Lỗi tải dữ liệu',
      saveError: 'Lỗi khi lưu'
    },
    memberGroups: {
      title: 'Nhóm hội viên',
      name: 'Tên nhóm',
      minSpent: 'Tối thiểu',
      discountPercent: 'Giảm giá %',
      isDefault: 'Mặc định',
      createdAt: 'Ngày tạo',
      create: 'Tạo nhóm',
      edit: 'Sửa nhóm',
      nameRequired: 'Vui lòng nhập tên nhóm',
      createSuccess: 'Tạo nhóm thành công',
      editSuccess: 'Cập nhật nhóm thành công',
      deleteConfirm: 'Xóa nhóm "{name}"?',
      deleteSuccess: 'Xóa thành công',
      loadError: 'Lỗi tải dữ liệu',
      saveError: 'Lỗi khi lưu'
    },
    members: {
      title: 'Hội viên',
      searchPlaceholder: 'Tìm theo tên / số điện thoại',
      add: 'Thêm hội viên',
      edit: 'Sửa hội viên',
      username: 'Tên đăng nhập',
      fullName: 'Họ tên',
      phone: 'SĐT',
      email: 'Email',
      balance: 'Số dư',
      bonus: 'Bonus',
      group: 'Nhóm',
      password: 'Mật khẩu',
      passwordPlaceholder: 'Để trống nếu không đổi',
      isActive: 'Kích hoạt',
      createdAt: 'Ngày tạo',
      lastLogin: 'Lần cuối đăng nhập',
      detail: 'Chi tiết',
      topUp: 'Nạp',
      topUpAmount: 'Nạp tiền',
      topUpMethod: 'Phương thức',
      resetPassword: 'Reset MK',
      resetPasswordConfirm: 'Nhập mật khẩu mới cho hội viên',
      resetPasswordRequired: 'Vui lòng nhập mật khẩu',
      cash: 'Tiền mặt',
      transfer: 'Chuyển khoản',
      eWallet: 'Ví điện tử',
      transactions: 'Giao dịch',
      sessions: 'Phiên chơi',
      amount: 'Số tiền',
      type: 'Loại',
      method: 'Phương thức',
      reference: 'Tham chiếu',
      form: {
        username: 'Tên đăng nhập',
        fullName: 'Họ tên',
        phone: 'Số điện thoại',
        email: 'Email',
        password: 'Mật khẩu',
        isActive: 'Kích hoạt',
        usernameRequired: 'Vui lòng nhập tên đăng nhập',
        passwordRequired: 'Vui lòng nhập mật khẩu',
        amountRequired: 'Vui lòng nhập số tiền',
        methodRequired: 'Vui lòng chọn phương thức'
      },
      messages: {
        addSuccess: 'Thêm hội viên thành công',
        editSuccess: 'Cập nhật hội viên thành công',
        deleteConfirm: 'Xóa hội viên "{name}"?',
        deleteSuccess: 'Xóa thành công',
        topUpSuccess: 'Nạp tiền thành công',
        resetPasswordSuccess: 'Reset mật khẩu thành công',
        topUpError: 'Lỗi khi nạp tiền',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi khi lưu',
        loadDetailError: 'Lỗi tải thông tin hội viên'
      }
    },
    sessions: {
      title: 'Phiên chơi đang hoạt động',
      activeSessions: 'Phiên chơi đang hoạt động',
      totalActive: 'Tổng số: {count} phiên đang hoạt động',
      machineCode: 'Mã máy',
      member: 'Hội viên',
      startTime: 'Bắt đầu',
      duration: 'Thời lượng',
      endTime: 'Kết thúc',
      status: 'Trạng thái',
      running: 'Đang chạy',
      ended: 'Kết thúc',
      end: 'Kết thúc',
      messages: {
        loadError: 'Lỗi tải dữ liệu',
        endConfirm: 'Kết thúc phiên chơi của "{member}" trên máy {code}?',
        endSuccess: 'Đã kết thúc phiên chơi'
      }
    },
    shifts: {
      title: 'Ca làm việc',
      openShift: 'Mở ca',
      closeShift: 'Đóng ca',
      handover: 'Bàn giao',
      startTime: 'Bắt đầu',
      endTime: 'Kết thúc',
      openingCash: 'Tiền đầu',
      closingCash: 'Tiền cuối',
      cashIn: 'Thu tiền (Cash In)',
      cashOut: 'Chi tiền (Cash Out)',
      reason: 'Lý do',
      amount: 'Số tiền',
      open: 'Đang mở',
      closed: 'Đã đóng',
      user: 'Người dùng',
      form: {
        openingCash: 'Tiền đầu ca',
        closingCash: 'Tiền cuối ca',
        cashIn: 'Thu tiền',
        cashOut: 'Chi tiền',
        reason: 'Lý do',
        handoverType: 'Loại bàn giao',
        amountRequired: 'Nhập số tiền',
        amountPlaceholder: 'Nhập số tiền',
        handoverTypePlaceholder: 'Chọn loại bàn giao'
      },
      messages: {
        loadError: 'Lỗi tải dữ liệu',
        openSuccess: 'Mở ca thành công',
        openError: 'Lỗi mở ca',
        closeSuccess: 'Đóng ca thành công',
        closeError: 'Lỗi đóng ca',
        handoverSuccess: 'Bàn giao ca thành công',
        handoverError: 'Lỗi bàn giao ca'
      }
    },
    orders: {
      title: 'Đơn hàng',
      newOrder: 'Đơn hàng mới',
      newOrderMessage: 'Có đơn hàng mới cần xử lý',
      orderCode: 'Mã đơn',
      search: 'Tìm kiếm',
      member: 'Hội viên',
      total: 'Tổng tiền',
      createdAt: 'Ngày tạo',
      completedAt: 'Ngày hoàn thành',
      status: 'Trạng thái',
      note: 'Ghi chú',
      product: 'Sản phẩm',
      products: 'Sản phẩm trong đơn',
      quantity: 'Số lượng',
      unitPrice: 'Đơn giá',
      subtotal: 'Thành tiền',
      createOrder: 'Tạo đơn hàng',
      selectProduct: 'Chọn sản phẩm',
      options: 'Tuỳ chọn',
      machineCode: 'Máy',
      operator: 'Thao tác',
      confirm: 'Xác nhận',
      cancel: 'Huỷ đơn',
      pay: 'Thanh toán',
      orderType: 'Loại',
      topup: 'Nạp tiền',
      product: 'Sản phẩm',
      approve: 'Duyệt nạp',
      reject: 'Từ chối',
      paymentMethod: 'Hình thức',
      previewTotal: 'Tạm tính',
      statusLabels: {
        pending: 'Chờ xác nhận',
        confirmed: 'Đã xác nhận',
        completed: 'Hoàn thành',
        cancelled: 'Đã hủy'
      },
      detail: 'Chi tiết',
      messages: {
        loadError: 'Lỗi tải dữ liệu',
        loadDetailError: 'Lỗi tải chi tiết',
        updateSuccess: 'Cập nhật trạng thái thành công',
        updateError: 'Lỗi cập nhật trạng thái',
        paymentConfirm: 'Xác nhận thanh toán đơn {code}?',
        cancelConfirm: 'Xác nhận huỷ đơn {code}?',
        paymentSuccess: 'Thanh toán thành công',
        createSuccess: 'Tạo đơn hàng thành công',
        createError: 'Lỗi tạo đơn hàng'
      }
    },
    products: {
      title: 'Sản phẩm',
      searchPlaceholder: 'Tìm kiếm...',
      add: 'Thêm sản phẩm',
      edit: 'Sửa sản phẩm',
      name: 'Tên',
      category: 'Danh mục',
      price: 'Giá',
      description: 'Mô tả',
      image: 'Hình ảnh',
      imageUrl: 'URL hình ảnh',
      supplier: 'Nhà cung cấp',
      isActive: 'Kích hoạt',
      hasStock: 'Theo dõi tồn kho',
      stockQuantity: 'Số lượng',
      currentStock: 'Tồn kho',
      ingredients: 'Nguyên liệu',
      unit: 'Đơn vị',
      minStock: 'Tồn tối thiểu',
      pricePerUnit: 'Giá/đơn vị',
      isRetail: 'Bán lẻ',
      nonRetail: 'Nguyên liệu',
      optionGroups: 'Tuỳ chọn',
      optionName: 'Tên tuỳ chọn',
      optionIngredientHint: 'Nguyên liệu tiêu hao khi chọn tuỳ chọn này',
      addOption: 'Thêm tuỳ chọn',
      optionItem: 'Mục tuỳ chọn',
      stockNote: 'Ghi chú',
      stockUnitPrice: 'Đơn giá',
      stockIn: 'Nhập kho',
      stockOut: 'Trừ kho',
      stockInDescription: 'Nhập kho sản phẩm',
      stockOutDescription: 'Xuất kho sản phẩm',
      addIngredient: 'Thêm nguyên liệu...',
      units: {
        piece: 'Cái',
        box: 'Hộp',
        bottle: 'Chai',
        can: 'Lon',
        kg: 'Kg',
        liter: 'Lít',
        pack: 'Gói',
        bag: 'Bịch',
        crate: 'Thùng',
        glass: 'Ly',
        cup: 'Cốc',
        plate: 'Đĩa',
        bowl: 'Tô',
        portion: 'Phần',
        serving: 'Suất',
        slice: 'Miếng',
        tablet: 'Viên',
        stick: 'Cây',
        rod: 'Que'
      },
      form: {
        name: 'Tên',
        category: 'Danh mục',
        price: 'Giá',
        nameRequired: 'Vui lòng nhập tên',
        priceRequired: 'Vui lòng nhập giá',
        categoryPlaceholder: 'Chọn danh mục',
        uploadImage: 'Tải ảnh lên',
        note: 'Ghi chú',
        quantityRequired: 'Vui lòng nhập số lượng'
      },
      messages: {
        addSuccess: 'Thêm thành công',
        editSuccess: 'Cập nhật thành công',
        deleteConfirm: 'Bạn có chắc muốn xóa sản phẩm này?',
        deleteSuccess: 'Xóa thành công',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi lưu dữ liệu',
        addIngredientSuccess: 'Đã thêm nguyên liệu',
        deleteIngredientSuccess: 'Đã xóa nguyên liệu',
        stockInSuccess: 'Nhập kho thành công',
        stockOutSuccess: 'Trừ kho thành công',
        noIngredients: 'Sản phẩm chưa có nguyên liệu'
      }
    },
    suppliers: {
      title: 'Nhà cung cấp',
      name: 'Tên',
      phone: 'SĐT',
      email: 'Email',
      addSupplier: 'Thêm nhà cung cấp',
      form: {
        nameRequired: 'Vui lòng nhập tên'
      },
      messages: {
        addSuccess: 'Thêm thành công',
        editSuccess: 'Cập nhật thành công',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi lưu dữ liệu'
      }
    },
    warehouses: {
      title: 'Kho',
      name: 'Tên',
      address: 'Địa chỉ',
      addWarehouse: 'Thêm kho',
      form: {
        nameRequired: 'Vui lòng nhập tên'
      },
      messages: {
        addSuccess: 'Thêm thành công',
        editSuccess: 'Cập nhật thành công',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi lưu dữ liệu'
      }
    },
    stockTransactions: {
      title: 'Giao dịch tồn kho',
      createTransaction: 'Tạo phiếu nhập/xuất',
      type: 'Loại',
      product: 'Sản phẩm',
      transactionType: 'Loại',
      importLabel: 'Nhập',
      exportLabel: 'Xuất',
      quantity: 'Số lượng',
      unitPrice: 'Đơn giá',
      totalPrice: 'Thành tiền',
      before: 'Trước',
      after: 'Sau',
      createdAt: 'Ngày tạo',
      form: {
        productRequired: 'Chọn sản phẩm',
        quantityRequired: 'Nhập số lượng'
      },
      messages: {
        createTransactionSuccess: 'Tạo phiếu thành công',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi lưu dữ liệu'
      }
    },
    reports: {
      title: 'Báo cáo',
      from: 'Từ',
      to: 'Đến',
      dailyRevenue: 'Doanh thu ngày',
      date: 'Ngày',
      totalRevenue: 'Tổng doanh thu',
      orderCount: 'Số đơn hàng',
      memberCount: 'Số hội viên',
      monthlyRevenue: 'Doanh thu tháng',
      month: 'Tháng',
      byMember: 'Theo hội viên',
      memberCode: 'Mã HV',
      totalSpent: 'Tổng chi tiêu',
      visitCount: 'Số lần',
      byMachine: 'Theo máy',
      machineName: 'Tên máy',
      revenue: 'Doanh thu',
      hours: 'Số giờ',
      sessionCount: 'Số phiên',
      member: 'Hội viên',
      machine: 'Máy',
      messages: {
        loadError: 'Lỗi tải báo cáo'
      }
    },
    bookings: {
      title: 'Đặt chỗ',
      customer: 'Khách hàng',
      machineCode: 'Mã máy',
      from: 'Từ',
      to: 'Đến',
      deposit: 'Tiền cọc',
      checkIn: 'Check-in',
      statusLabels: {
        pending: 'Chờ xác nhận',
        confirmed: 'Đã xác nhận',
        checkedIn: 'Đã check-in',
        cancelled: 'Đã hủy',
        noShow: 'Không đến'
      },
      messages: {
        loadError: 'Lỗi tải dữ liệu',
        checkInConfirm: 'Check-in cho "{customer}" tại máy {code}?',
        checkInSuccess: 'Check-in thành công',
        cancelConfirm: 'Hủy đặt chỗ của "{customer}"?',
        cancelSuccess: 'Đã hủy đặt chỗ'
      }
    },
    categories: {
      title: 'Danh mục',
      name: 'Tên',
      icon: 'Biểu tượng',
      order: 'Thứ tự',
      parent: 'Danh mục cha',
      searchPlaceholder: 'Tìm danh mục...',
      add: 'Thêm danh mục',
      edit: 'Sửa danh mục',
      iconPlaceholder: 'Chọn biểu tượng',
      parentPlaceholder: 'Không có (cấp cao nhất)',
      form: {
        nameRequired: 'Vui lòng nhập tên danh mục'
      },
      messages: {
        addSuccess: 'Thêm danh mục thành công',
        editSuccess: 'Cập nhật danh mục thành công',
        saveError: 'Lỗi lưu danh mục',
        deleteConfirm: 'Xóa danh mục này?',
        deleteSuccess: 'Xóa danh mục thành công'
      }
    },
    promotions: {
      title: 'Khuyến mãi',
      searchPlaceholder: 'Tìm khuyến mãi...',
      add: 'Thêm khuyến mãi',
      edit: 'Sửa khuyến mãi',
      name: 'Tên',
      type: 'Loại',
      priority: 'Priority',
      isActive: 'Kích hoạt',
      validPeriod: 'Hiệu lực',
      conditions: 'Điều kiện',
      rewards: 'Phần thưởng',
      luckySpinRewards: 'Lucky Spin Rewards',
      value: 'Giá trị',
      probability: 'Xác suất',
      form: {
        name: 'Tên',
        type: 'Loại',
        typePlaceholder: 'VD: percentage, fixed, combo',
        nameRequired: 'Vui lòng nhập tên',
        typeRequired: 'Vui lòng nhập loại'
      },
      messages: {
        addSuccess: 'Thêm khuyến mãi thành công',
        editSuccess: 'Cập nhật khuyến mãi thành công',
        deleteConfirm: 'Xóa khuyến mãi "{name}"?',
        deleteSuccess: 'Xóa thành công',
        loadError: 'Lỗi tải dữ liệu',
        loadRewardsError: 'Lỗi tải rewards',
        saveError: 'Lỗi khi lưu'
      }
    },
    chat: {
      title: 'Trò chuyện',
      rooms: 'Hội thoại',
      with: 'với',
      newRoom: 'Tạo mới',
      noMessages: 'Chưa có tin nhắn',
      selectRoom: 'Chọn hội thoại',
      inputPlaceholder: 'Nhập tin nhắn...',
      send: 'Gửi',
      createRoom: 'Tạo hội thoại mới',
      recipients: 'Người nhận',
      selectRecipients: 'Chọn người nhận',
      messages: {
        selectAtLeastOne: 'Chọn ít nhất 1 người',
        loadRoomsError: 'Lỗi tải hội thoại',
        loadMessagesError: 'Lỗi tải tin nhắn',
        sendError: 'Lỗi gửi tin nhắn',
        createSuccess: 'Tạo hội thoại thành công',
        createError: 'Lỗi tạo hội thoại'
      },
      newMessage: 'Tin nhắn mới'
    },
    settings: {
      title: 'Cài đặt',
      general: 'Chung',
      billing: 'Tính giờ',
      printing: 'Máy in',
      invoice: 'Hóa đơn',
      storeName: 'Tên cửa hàng',
      address: 'Địa chỉ',
      phone: 'Số điện thoại',
      email: 'Email',
      timezone: 'Múi giờ',
      pricing: 'Giá',
      pricePerHour: 'Giá mỗi giờ (VNĐ)',
      pricePerMinute: 'Giá mỗi phút (VNĐ)',
      minMinutes: 'Phút tối thiểu',
      hourlyDiscount: 'Giảm giờ theo giờ',
      limits: 'Giới hạn',
      maxBookingsPerDay: 'Giới hạn đặt chỗ / ngày',
      maxBookingsPerMember: 'Giới hạn đặt chỗ / hội viên',
      cancelBeforeMinutes: 'Hủy đặt chỗ trước (phút)',
      maxDebt: 'Nợ tối đa (VNĐ)',
      printerType: 'Loại máy in',
      thermal: 'Nhiệt',
      laser: 'Laser',
      inkjet: 'Inkjet',
      printerName: 'Tên máy in',
      paperSize: 'Kích thước giấy',
      paperSizePlaceholder: 'VD: 80mm',
      autoPrint: 'Tự động in',
      invoiceTitle: 'Tiêu đề hóa đơn',
      invoiceFooter: 'Chân trang hóa đơn',
      taxCode: 'Mã số thuế',
      invoiceStartNumber: 'Số hóa đơn bắt đầu',
      showLogo: 'Hiển thị logo',
      messages: {
        saveSuccess: 'Lưu cài đặt thành công',
        saveError: 'Lỗi lưu cài đặt'
      }
    },
    audit: {
      title: 'Nhật ký hoạt động',
      action: 'Hành động',
      target: 'Đối tượng',
      from: 'Từ',
      to: 'Đến',
      id: 'ID',
      user: 'Người dùng',
      description: 'Mô tả',
      time: 'Thời gian',
      actionLabels: {
        create: 'Tạo',
        update: 'Cập nhật',
        delete: 'Xóa',
        topup: 'Nạp tiền',
        refund: 'Hoàn tiền',
        purchase: 'Mua',
        pay: 'Thanh toán',
        cancel: 'Hủy'
      },
      entityLabels: {
        member: 'hội viên',
        machine: 'máy',
        order: 'đơn hàng',
        product: 'sản phẩm',
        category: 'danh mục',
        combo: 'gói dịch vụ',
        promotion: 'khuyến mãi',
        booking: 'đặt chỗ',
        user: 'người dùng',
        role: 'vai trò',
        store: 'cửa hàng'
      },
      messages: {
        loadError: 'Lỗi tải nhật ký'
      }
    },
    stores: {
      title: 'Cửa hàng',
      searchPlaceholder: 'Tìm kiếm...',
      add: 'Thêm cửa hàng',
      edit: 'Sửa cửa hàng',
      name: 'Tên',
      code: 'Mã',
      phone: 'SĐT',
      address: 'Địa chỉ',
      isActive: 'Kích hoạt',
      form: {
        nameRequired: 'Vui lòng nhập tên',
        codeRequired: 'Vui lòng nhập mã'
      },
      messages: {
        addSuccess: 'Thêm thành công',
        editSuccess: 'Cập nhật thành công',
        deleteConfirm: 'Bạn có chắc muốn xóa cửa hàng này?',
        deleteSuccess: 'Xóa thành công',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi lưu dữ liệu'
      }
    },
    combos: {
      title: 'Gói dịch vụ',
      searchPlaceholder: 'Tìm combo...',
      add: 'Thêm combo',
      edit: 'Sửa combo',
      name: 'Tên combo',
      type: 'Loại',
      fixedSlot: 'Fixed slot',
      prepaid: 'Prepaid',
      price: 'Giá',
      isActive: 'Kích hoạt',
      from: 'Từ',
      to: 'Đến',
      addSlot: '+ Thêm slot',
      minutes: 'Số phút',
      form: {
        name: 'Tên combo',
        type: 'Loại combo',
        price: 'Giá',
        minutes: 'Số phút',
        nameRequired: 'Vui lòng nhập tên combo',
        typeRequired: 'Vui lòng chọn loại',
        priceRequired: 'Vui lòng nhập giá',
        minutesRequired: 'Vui lòng nhập số phút'
      },
      messages: {
        addSuccess: 'Thêm combo thành công',
        editSuccess: 'Cập nhật combo thành công',
        deleteConfirm: 'Xóa combo "{name}"?',
        deleteSuccess: 'Xóa thành công',
        loadError: 'Lỗi tải dữ liệu',
        saveError: 'Lỗi khi lưu'
      }
    },
    backups: {
      title: 'Sao lưu & phục hồi',
      createBackup: 'Tạo sao lưu',
      fileName: 'Tên file',
      size: 'Kích thước',
      status: 'Trạng thái',
      createdAt: 'Ngày tạo',
      completed: 'Hoàn thành',
      running: 'Đang chạy',
      failed: 'Thất bại',
      restore: 'Phục hồi',
      messages: {
        loadError: 'Lỗi tải danh sách sao lưu',
        createConfirm: 'Tạo bản sao lưu mới? Quá trình này có thể mất vài phút.',
        creating: 'Đang tạo sao lưu...',
        restoreConfirm: 'Phục hồi dữ liệu từ bản sao lưu "{file}"? Dữ liệu hiện tại sẽ bị thay thế.',
        restoring: 'Đang phục hồi dữ liệu...',
        deleteComingSoon: 'Tính năng xóa bản sao lưu đang phát triển'
      }
    },
    transactions: {
      title: 'Lịch sử giao dịch',
      from: 'Từ',
      to: 'Đến',
      typePlaceholder: 'Loại giao dịch',
      all: 'Tất cả',
      searchPlaceholder: 'Tìm theo tên / SĐT / tài khoản',
      date: 'Thời gian',
      member: 'Hội viên',
      type: 'Loại',
      amount: 'Số tiền',
      balanceBefore: 'Số dư trước',
      balanceAfter: 'Số dư sau',
      paymentMethod: 'Phương thức',
      description: 'Mô tả',
      createdBy: 'Người thực hiện',
      topup: 'Nạp tiền',
      sessionFee: 'Phí chơi',
      refund: 'Hoàn tiền',
      cancel: 'Hủy',
      comboPurchase: 'Mua combo',
      messages: {
        loadError: 'Lỗi tải lịch sử giao dịch'
      }
    }
  }
};

export default local;
