declare namespace Api {
  namespace Auth {
    interface LoginToken {
      access_token: string;
      refresh_token: string;
      user: UserInfo;
      /** Mật khẩu đúng nhưng tài khoản bật xác thực 2 bước: chưa có token, hỏi mã rồi gửi kèm challenge. */
      need_totp?: boolean;
      challenge?: string;
    }

    interface TwoFactorStatus {
      /** Chủ quán đã bật tính năng ở Cài đặt > Tính năng. */
      feature_enabled: boolean;
      enabled: boolean;
      enabled_at?: string;
      recovery_left: number;
    }

    interface TwoFactorSetup {
      secret: string;
      otpauth_url: string;
      /** Ảnh QR dạng data URL. */
      qr: string;
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
