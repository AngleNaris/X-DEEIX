import type { LoginOptionsData, LoginPageSettings } from "@/shared/api/auth.types";
import { ApiError } from "@/shared/api/http-client";
import { DEFAULT_AUTH_NEXT_PATH } from "@/shared/auth/local-path";

export type LoginMode = "login" | "register" | "reset-password";
export type ProviderAuthIntent = "login" | "register";

export const DEFAULT_LOGIN_SETTINGS: LoginPageSettings = {
  defaultNextPath: DEFAULT_AUTH_NEXT_PATH,
};

export const DEFAULT_LOGIN_OPTIONS: LoginOptionsData = {
  usernameEnabled: true,
  emailEnabled: true,
  emailRegistrationEnabled: true,
  emailVerificationEnabled: false,
  passwordResetEnabled: false,
  turnstileRegistrationEnabled: false,
  turnstileSiteKey: "",
  providerAuthBridge: {
    callbackBaseURL: "",
    enabled: false,
    protocolVersion: 1,
  },
  providers: [],
};

export const TWO_FACTOR_CHALLENGE_STORAGE_KEY = "deeix-chat:2fa:challenge";
export const TWO_FACTOR_METHODS_STORAGE_KEY = "deeix-chat:2fa:methods";

export function normalizeTwoFactorInput(value: string): string {
  return value.replace(/[^a-zA-Z0-9-]/g, "").slice(0, 32);
}

export function normalizeRegisterCode(value: string): string {
  return value.replace(/\D/g, "").slice(0, 6);
}

export function isTwoFactorChallengeExpired(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401 && error.message === "two factor challenge expired";
}


