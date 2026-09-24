import "server-only";

import { cookies } from "next/headers";

import type { components } from "@/lib/api/generated";

export type CandidateProfile = components["schemas"]["Profile"];
export type CandidateSkill = components["schemas"]["Skill"];
type Tokens = components["schemas"]["TokenOutputBody"];

const apiBaseUrl = process.env.API_BASE_URL ?? "http://127.0.0.1:8080";
const cookieOptions = {
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "lax" as const,
  path: "/",
};

function secondsUntil(value: string) {
  return Math.max(0, Math.floor((Date.parse(value) - Date.now()) / 1000));
}

export async function storeTokens(tokens: Tokens) {
  const jar = await cookies();
  jar.set("hr_access", tokens.access_token, {
    ...cookieOptions,
    maxAge: secondsUntil(tokens.access_expires_at),
  });
  jar.set("hr_refresh", tokens.refresh_token, {
    ...cookieOptions,
    maxAge: secondsUntil(tokens.refresh_expires_at),
  });
}

export async function clearTokens() {
  const jar = await cookies();
  jar.delete("hr_access");
  jar.delete("hr_refresh");
}

export async function apiRequest(path: string, init: RequestInit = {}) {
  return fetch(`${apiBaseUrl}${path}`, { ...init, cache: "no-store" });
}

async function refreshAccessToken() {
  const refreshToken = (await cookies()).get("hr_refresh")?.value;
  if (!refreshToken) return null;
  const response = await apiRequest("/api/v1/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  if (!response.ok) return null;
  const tokens = (await response.json()) as Tokens;
  await storeTokens(tokens);
  return tokens.access_token;
}

export async function authenticatedRequest(
  path: string,
  init: RequestInit = {},
  allowRefresh = false,
) {
  let accessToken: string | null | undefined = (await cookies()).get(
    "hr_access",
  )?.value;
  if (!accessToken && allowRefresh) accessToken = await refreshAccessToken();
  if (!accessToken) return null;

  const request = () =>
    apiRequest(path, {
      ...init,
      headers: { ...init.headers, Authorization: `Bearer ${accessToken}` },
    });
  let response = await request();
  if (response.status === 401 && allowRefresh) {
    accessToken = await refreshAccessToken();
    if (!accessToken) return response;
    response = await request();
  }
  return response;
}

export async function getCandidateData() {
  const [profileResponse, skillsResponse] = await Promise.all([
    authenticatedRequest("/api/v1/profile"),
    authenticatedRequest("/api/v1/profile/skills"),
  ]);
  if (!profileResponse?.ok || !skillsResponse?.ok) return null;
  const profile = (await profileResponse.json()) as CandidateProfile;
  const body = (await skillsResponse.json()) as {
    skills: CandidateSkill[] | null;
  };
  return { profile, skills: body.skills ?? [] };
}
