"use server";

import { redirect } from "next/navigation";

import { storeTokens } from "@/lib/api/server";

const apiBaseUrl = process.env.API_BASE_URL ?? "http://127.0.0.1:8080";

export async function register(formData: FormData) {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");
  let status = 0;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/auth/register`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
      cache: "no-store",
    });
    status = response.status;
  } catch {}
  if (status !== 201) {
    const error =
      status === 409
        ? "duplicate"
        : status === 400
          ? "validation"
          : status === 429
            ? "limited"
            : "unavailable";
    redirect(`/register?error=${error}`);
  }
  redirect("/register?sent=1");
}

export async function signIn(formData: FormData) {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");
  let status = 0;
  let tokens: { access_token: string; refresh_token: string } | undefined;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email, password }),
      cache: "no-store",
    });
    status = response.status;
    if (response.ok) tokens = await response.json();
  } catch {}
  if (status !== 200 || !tokens?.access_token || !tokens.refresh_token) {
    const error =
      status === 401 ? "invalid" : status === 429 ? "limited" : "unavailable";
    redirect(`/sign-in?error=${error}`);
  }
  await storeTokens(tokens as Parameters<typeof storeTokens>[0]);
  redirect("/onboarding/profile");
}

export async function requestPasswordReset(formData: FormData) {
  const email = String(formData.get("email") ?? "");
  let available = false;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/auth/forgot-password`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
      cache: "no-store",
    });
    available = response.ok;
  } catch {}
  if (!available) redirect("/forgot-password?error=unavailable");
  redirect("/forgot-password?sent=1");
}

export async function resetPassword(formData: FormData) {
  const token = String(formData.get("token") ?? "");
  const password = String(formData.get("password") ?? "");
  const confirmation = String(formData.get("confirmPassword") ?? "");
  const query = new URLSearchParams({ token });
  if (!token || password.length < 12 || password !== confirmation) {
    query.set("error", "validation");
    redirect(`/reset-password?${query}`);
  }
  let status = 0;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/auth/reset-password`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token, password }),
      cache: "no-store",
    });
    status = response.status;
  } catch {}
  if (status !== 204) {
    query.set("error", status === 400 ? "invalid" : "unavailable");
    redirect(`/reset-password?${query}`);
  }
  redirect("/reset-password?done=1");
}

export async function verifyEmail(formData: FormData) {
  const token = String(formData.get("token") ?? "");
  if (!token) redirect("/verify-email?error=invalid");
  let status = 0;
  try {
    const response = await fetch(`${apiBaseUrl}/api/v1/auth/verify-email`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
      cache: "no-store",
    });
    status = response.status;
  } catch {}
  if (status !== 204) {
    const query = new URLSearchParams({
      token,
      error: status === 400 ? "invalid" : "unavailable",
    });
    redirect(`/verify-email?${query}`);
  }
  redirect("/verify-email?done=1");
}
