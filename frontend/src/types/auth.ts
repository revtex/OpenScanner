// Authentication request/response shapes.

export interface LoginResponse {
  token: string;
  user: {
    id: number;
    username: string;
    role: string;
  };
  passwordNeedChange: boolean;
}

export interface RefreshResponse {
  token: string;
  user: {
    id: number;
    username: string;
    role: string;
  };
}

export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
}
