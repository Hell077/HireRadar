"use server";

import type { components } from "@/lib/api/generated";
import { authenticatedRequest } from "@/lib/api/server";

type UploadBody = components["schemas"]["ResumeUploadOutputBody"];

export async function createResumeUpload(fileName: string, size: number) {
  const response = await authenticatedRequest(
    "/api/v1/resumes/upload-url",
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        file_name: fileName,
        content_type: "application/pdf",
        size,
      }),
    },
    true,
  ).catch(() => null);
  if (!response?.ok) return null;
  return (await response.json()) as UploadBody;
}

export async function completeResumeUpload(id: string) {
  const response = await authenticatedRequest(
    `/api/v1/resumes/${encodeURIComponent(id)}/complete`,
    { method: "POST" },
    true,
  ).catch(() => null);
  return Boolean(response?.ok);
}
