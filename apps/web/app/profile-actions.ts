"use server";

import { cookies } from "next/headers";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import {
  apiRequest,
  authenticatedRequest,
  clearTokens,
} from "@/lib/api/server";

function text(formData: FormData, key: string) {
  return String(formData.get(key) ?? "").trim();
}

function profilePayload(formData: FormData) {
  const salary = text(formData, "salary");
  return {
    first_name: text(formData, "firstName"),
    last_name: text(formData, "lastName"),
    country: text(formData, "country"),
    city: text(formData, "city"),
    timezone: text(formData, "timezone"),
    experience_years: Number(text(formData, "experience")),
    seniority: text(formData, "seniority"),
    ...(salary && text(formData, "currency")
      ? {
          desired_salary: {
            amount: Number(salary),
            currency: text(formData, "currency"),
          },
        }
      : {}),
  };
}

async function saveProfile(formData: FormData) {
  return authenticatedRequest(
    "/api/v1/profile",
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(profilePayload(formData)),
    },
    true,
  );
}

export async function updateProfile(formData: FormData) {
  const response = await saveProfile(formData).catch(() => null);
  if (!response?.ok) redirect("/?profileError=1");
  revalidatePath("/");
  redirect("/?profileUpdated=1");
}

export async function saveOnboardingProfile(formData: FormData) {
  const response = await saveProfile(formData).catch(() => null);
  if (!response?.ok) redirect("/onboarding/profile?error=1");
  revalidatePath("/");
  redirect("/onboarding/resume");
}

export async function updateSkills(formData: FormData) {
  const skills = formData
    .getAll("skills")
    .map(String)
    .map((name) => ({ name: name.trim() }))
    .filter(({ name }) => name);
  const response = await authenticatedRequest(
    "/api/v1/profile/skills",
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ skills }),
    },
    true,
  ).catch(() => null);
  if (!response?.ok) redirect("/?skillsError=1");
  revalidatePath("/");
  redirect("/?skillsUpdated=1");
}

export async function signOut() {
  const refreshToken = (await cookies()).get("hr_refresh")?.value;
  if (refreshToken) {
    await apiRequest("/api/v1/auth/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    }).catch(() => undefined);
  }
  await clearTokens();
  redirect("/sign-in");
}
