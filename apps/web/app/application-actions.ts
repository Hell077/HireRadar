"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { authenticatedRequest } from "@/lib/api/server";

function field(data: FormData, name: string) {
  return String(data.get(name) ?? "").trim();
}

export async function answerApplicationQuestion(data: FormData) {
  const id = field(data, "questionId");
  const answer = field(data, "answer");
  if (!id || !answer) redirect("/applications?error=1");
  const response = await authenticatedRequest(
    `/api/v1/application-questions/${encodeURIComponent(id)}/answer`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answer }),
    },
    true,
  ).catch(() => null);
  revalidatePath("/applications");
  redirect(response?.ok ? "/applications?answered=1" : "/applications?error=1");
}

export async function retryApplication(data: FormData) {
  const id = field(data, "applicationId");
  const response = id
    ? await authenticatedRequest(
        `/api/v1/applications/${encodeURIComponent(id)}/retry`,
        { method: "POST" },
        true,
      ).catch(() => null)
    : null;
  revalidatePath("/applications");
  redirect(response?.ok ? "/applications?retried=1" : "/applications?error=1");
}

export async function cancelApplication(data: FormData) {
  const id = field(data, "applicationId");
  const response = id
    ? await authenticatedRequest(
        `/api/v1/applications/${encodeURIComponent(id)}/cancel`,
        { method: "POST" },
        true,
      ).catch(() => null)
    : null;
  revalidatePath("/applications");
  redirect(
    response?.ok ? "/applications?cancelled=1" : "/applications?error=1",
  );
}

export async function saveApplicationProfile(data: FormData) {
  const customAnswersText = field(data, "customAnswers") || "{}";
  let custom_answers: Record<string, string>;
  try {
    const parsed: unknown = JSON.parse(customAnswersText);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
      throw new Error("invalid");
    custom_answers = Object.fromEntries(
      Object.entries(parsed).map(([key, value]) => [key, String(value)]),
    );
  } catch {
    redirect("/applications/profile?error=1");
  }
  let work_authorization: { country: string; status: string }[];
  try {
    const parsed: unknown = JSON.parse(
      field(data, "workAuthorization") || "[]",
    );
    if (!Array.isArray(parsed)) throw new Error("invalid");
    work_authorization = parsed.map((entry) => {
      if (
        !entry ||
        typeof entry !== "object" ||
        !("country" in entry) ||
        !("status" in entry)
      )
        throw new Error("invalid");
      return { country: String(entry.country), status: String(entry.status) };
    });
  } catch {
    redirect("/applications/profile?error=1");
  }
  const amount = Number(field(data, "salaryAmount"));
  const currency = field(data, "salaryCurrency");
  const notice = Number(field(data, "noticePeriod"));
  const body = {
    first_name: field(data, "firstName"),
    last_name: field(data, "lastName"),
    email: field(data, "email"),
    phone: field(data, "phone"),
    country: field(data, "country"),
    city: field(data, "city"),
    address: field(data, "address"),
    linkedin_url: field(data, "linkedin"),
    github_url: field(data, "github"),
    website_url: field(data, "website"),
    resume_id: field(data, "resumeId") || undefined,
    expected_salary:
      amount > 0 && currency
        ? { amount: Math.round(amount), currency }
        : undefined,
    notice_period_days:
      Number.isInteger(notice) && notice >= 0 ? notice : undefined,
    work_authorization,
    custom_answers,
  };
  const response = await authenticatedRequest(
    "/api/v1/application-profile",
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    },
    true,
  ).catch(() => null);
  revalidatePath("/applications/profile");
  redirect(
    response?.ok
      ? "/applications/profile?saved=1"
      : "/applications/profile?error=1",
  );
}
