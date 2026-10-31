declare namespace Api {
  namespace Auth {
    interface LoginToken {
      access_token: string;
      refresh_token: string;
      user: UserInfo;
      /** Mật khẩu đúng nhưng tài khoản bật xác thực 2 bước: chưa có token, hỏi mã rồi gửi kèm challenge. */
      need_totp?: boolean;
      challenge?: string;
      /** Quán bắt buộc 2 bước mà tài khoản chưa cài: quét `setup.qr`, gửi mã kèm challenge là cài xong. */
      need_setup?: boolean;
      setup?: TwoFactorSetup;
      /** 10 mã dự phòng, chỉ có ở lần cài xong ngay lúc đăng nhập. */
      recovery_codes?: string[];
    }

    interface TwoFactorStatus {
      /** Chủ quán bắt buộc 2 bước (Cài đặt > Tính năng): không tự tắt được. */
      required: boolean;
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
