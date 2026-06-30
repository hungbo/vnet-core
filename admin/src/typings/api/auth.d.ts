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
    }
  }
}
