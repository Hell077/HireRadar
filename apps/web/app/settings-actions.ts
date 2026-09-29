"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { authenticatedRequest } from "@/lib/api/server";
import { normalizePositions } from "@/lib/profile/positions";

const values = (data: FormData, name: string) =>
  data.getAll(name).map(String).filter(Boolean);
const number = (data: FormData, name: string, fallback: number) => {
  const parsed = Number(data.get(name));
  return Number.isFinite(parsed) ? parsed : fallback;
};

async function put(path: string, body: unknown) {
  return authenticatedRequest(
    path,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
    true,
  );
}

export async function saveSettings(formData: FormData) {
  const sourceIDs = values(formData, "source");
  const allSourceIDs = values(formData, "allSource");
  const timezone = String(formData.get("timezone") ?? "UTC");
  const minimumScore = number(formData, "minimumScore", 70);
  const enabled = formData.get("notificationsEnabled") === "on";

  const requests = await Promise.all([
    put("/api/v1/profile/positions", {
      positions: normalizePositions(formData.getAll("positions").map(String)),
    }),
    put("/api/v1/profile/preferences", {
      remote_policies: values(formData, "remotePolicy"),
      employment_types: values(formData, "employmentType").flatMap((value) =>
        value === "project" ? ["contract", "freelance", "b2b"] : [value],
      ),
      allowed_regions: values(formData, "region"),
      excluded_countries: values(formData, "excludedCountry"),
      minimum_match_score: minimumScore,
      maximum_job_age_days: number(formData, "maximumJobAge", 30),
      notifications_enabled: enabled,
    }),
    put("/api/v1/profile/sources", {
      sources: allSourceIDs.map((source_id) => ({
        source_id,
        enabled: sourceIDs.includes(source_id),
      })),
    }),
    put("/api/v1/telegram/preferences", {
      enabled,
      minimum_score: minimumScore,
      immediate: formData.get("immediate") === "on",
      digest_enabled: formData.get("digestEnabled") === "on",
      timezone,
      max_per_day: number(formData, "maxPerDay", 10),
    }),
  ]).catch(() => []);

  revalidatePath("/settings");
  redirect(
    requests.length === 4 && requests.every((response) => response?.ok)
      ? "/settings?saved=1"
      : "/settings?error=1",
  );
}

export async function connectTelegram() {
  const response = await authenticatedRequest(
    "/api/v1/telegram/link",
    { method: "POST" },
    true,
  ).catch(() => null);
  if (!response?.ok) redirect("/settings?telegramError=1");
  const body = (await response.json()) as { url: string };
  redirect(body.url);
}

export async function disconnectTelegram() {
  const response = await authenticatedRequest(
    "/api/v1/telegram",
    { method: "DELETE" },
    true,
  ).catch(() => null);
  revalidatePath("/settings");
  revalidatePath("/");
  redirect(
    response?.ok ? "/settings?disconnected=1" : "/settings?telegramError=1",
  );
}
