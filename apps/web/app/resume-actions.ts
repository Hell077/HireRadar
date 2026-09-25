"use server";

import type { components } from "@/lib/api/generated";
import { authenticatedRequest } from "@/lib/api/server";

type UploadBody = components["schemas"]["ResumeUploadOutputBody"];
export type ResumeSuggestion = components["schemas"]["Suggestion"];
export type ParsedResume = components["schemas"]["ParsedResume"];

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

export async function getLatestResumeAnalysis() {
  const listResponse = await authenticatedRequest("/api/v1/resumes", {}, true).catch(() => null);
  if (!listResponse?.ok) return null;
  const list = (await listResponse.json()) as { resumes: components["schemas"]["Resume"][] | null };
  const resume = [...(list.resumes ?? [])].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at))[0];
  if (!resume) return null;
  const response = await authenticatedRequest(`/api/v1/resumes/${encodeURIComponent(resume.id)}/analysis`, {}, true).catch(() => null);
  if (!response?.ok) return { resume, analysis: null, suggestions: [] as ResumeSuggestion[] };
  const body = (await response.json()) as components["schemas"]["ResumeAnalysisOutputBody"];
  return { resume, analysis: body.analysis, suggestions: body.suggestions ?? [] };
}

export async function reviewResumeSuggestion(resumeID: string, suggestionID: string, accept: boolean) {
  const action = accept ? "accept" : "reject";
  const response = await authenticatedRequest(
    `/api/v1/resumes/${encodeURIComponent(resumeID)}/suggestions/${encodeURIComponent(suggestionID)}/${action}`,
    { method: "POST" },
    true,
  ).catch(() => null);
  return Boolean(response?.ok);
}
