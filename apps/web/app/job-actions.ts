"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { authenticatedRequest } from "@/lib/api/server";

export async function removeSavedJob(formData: FormData) {
  const id = String(formData.get("jobId") ?? "");
  const response = await authenticatedRequest(
    `/api/v1/saved-jobs/${encodeURIComponent(id)}`,
    { method: "DELETE" },
    true,
  ).catch(() => null);
  revalidatePath("/saved");
  redirect(response?.ok ? "/saved?removed=1" : "/saved?error=1");
}

export async function markJobApplied(formData: FormData) {
  const id = String(formData.get("jobId") ?? "");
  const requestedReturnTo = String(formData.get("returnTo") ?? "/jobs");
  const returnTo =
    requestedReturnTo.startsWith("/") && !requestedReturnTo.startsWith("//")
      ? requestedReturnTo
      : "/jobs";
  const response = await authenticatedRequest(
    `/api/v1/jobs/${encodeURIComponent(id)}/feedback`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "applied" }),
    },
    true,
  ).catch(() => null);
  revalidatePath("/jobs");
  redirect(
    `${returnTo}${returnTo.includes("?") ? "&" : "?"}${response?.ok ? "applied=1" : "error=1"}`,
  );
}

export async function hideJob(formData: FormData) {
  const id = String(formData.get("jobId") ?? "");
  await authenticatedRequest(
    `/api/v1/jobs/${encodeURIComponent(id)}/feedback`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "hide" }),
    },
    true,
  ).catch(() => null);
  revalidatePath("/jobs");
  redirect("/jobs");
}
