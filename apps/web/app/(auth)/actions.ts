"use server";

import { redirect } from "next/navigation";

const apiBaseUrl = process.env.API_BASE_URL ?? "http://127.0.0.1:8080";

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
