declare namespace Api {
  namespace Auth {
    interface LoginToken {
      access_token: string;
      refresh_token: string;
      user: UserInfo;
    }

    interface UserInfo {
      id: string;
      username: string;
      full_name: string;
      email: string;
      phone: string;
      avatar_url: string;
      role: string;
      permissions: string[];
      /** Máy chủ báo tài khoản còn dùng mật khẩu mặc định: giao diện phải buộc đổi. */
      must_change_password?: boolean;
    }
  }
}
