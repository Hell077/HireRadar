import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";
import { AppHeader } from "@/components/app-header";
import { MobileNavigation } from "@/components/mobile-navigation";
import { authenticatedRequest } from "@/lib/api/server";
import type { components } from "@/lib/api/generated";
import { saveApplicationProfile } from "@/app/application-actions";

export default async function ApplicationProfilePage({
  searchParams,
}: {
  searchParams: Promise<{ saved?: string; error?: string }>;
}) {
  const [profileResponse, resumesResponse, t, params] = await Promise.all([
    authenticatedRequest("/api/v1/application-profile", {}, true),
    authenticatedRequest("/api/v1/resumes", {}, true),
    getTranslations("applications"),
    searchParams,
  ]);
  if (!profileResponse) redirect("/sign-in");
  if (profileResponse.status === 401) redirect("/sign-in");
  const profile = profileResponse.ok
    ? ((await profileResponse.json()) as components["schemas"]["ApplicationProfile"])
    : null;
  const resumes = resumesResponse?.ok
    ? ((
        (await resumesResponse.json()) as components["schemas"]["ResumesOutputBody"]
      ).resumes ?? [])
    : [];
  const input = (label: string, name: string, value = "", type = "text") => (
    <label className="block text-sm font-medium">
      {label}
      <input
        className="mt-2 h-11 w-full rounded-md border bg-background px-3"
        name={name}
        type={type}
        defaultValue={value}
      />
    </label>
  );
  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="applications" />
      <main className="mx-auto w-full max-w-3xl px-4 py-8 sm:px-6 md:py-12">
        <Link href="/applications" className="text-sm text-primary underline">
          {t("backToApplications")}
        </Link>
        <h1 className="mt-4 text-3xl font-bold tracking-tight">
          {t("profileTitle")}
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {t("profileDescription")}
        </p>
        {params.saved ? (
          <p className="mt-5 rounded-lg bg-success-soft p-3 text-sm text-success">
            {t("profileSaved")}
          </p>
        ) : null}
        {params.error ? (
          <p
            role="alert"
            className="mt-5 rounded-lg bg-destructive-soft p-3 text-sm text-destructive"
          >
            {t("actionError")}
          </p>
        ) : null}
        <form
          action={saveApplicationProfile}
          className="mt-6 space-y-6 rounded-xl border bg-card p-5 sm:p-7"
        >
          <div className="grid gap-4 sm:grid-cols-2">
            {input(t("firstName"), "firstName", profile?.first_name)}
            {input(t("lastName"), "lastName", profile?.last_name)}
            {input(t("email"), "email", profile?.email, "email")}
            {input(t("phone"), "phone", profile?.phone, "tel")}
            {input(t("country"), "country", profile?.country)}
            {input(t("city"), "city", profile?.city)}
            {input(t("address"), "address", profile?.address)}
            {input(t("linkedin"), "linkedin", profile?.linkedin_url, "url")}
            {input(t("github"), "github", profile?.github_url, "url")}
            {input(t("website"), "website", profile?.website_url, "url")}
            <label className="block text-sm font-medium">
              {t("resume")}
              <select
                className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                name="resumeId"
                defaultValue={profile?.resume_id ?? ""}
              >
                <option value="">{t("noResume")}</option>
                {resumes.map((resume) => (
                  <option key={resume.id} value={resume.id}>
                    {resume.file_name}
                  </option>
                ))}
              </select>
            </label>
            <label className="block text-sm font-medium">
              {t("noticePeriod")}
              <input
                className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                name="noticePeriod"
                type="number"
                min="0"
                defaultValue={profile?.notice_period_days ?? ""}
              />
            </label>
            <label className="block text-sm font-medium">
              {t("salaryAmount")}
              <input
                className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                name="salaryAmount"
                type="number"
                min="0"
                defaultValue={profile?.expected_salary?.amount ?? ""}
              />
            </label>
            {input(
              t("salaryCurrency"),
              "salaryCurrency",
              profile?.expected_salary?.currency ?? "USD",
            )}
          </div>
          <label className="block text-sm font-medium">
            {t("customAnswers")}
            <p className="mt-1 text-xs font-normal text-muted-foreground">
              {t("customAnswersHelp")}
            </p>
            <textarea
              className="mt-2 min-h-28 w-full rounded-md border bg-background p-3 font-mono text-sm"
              name="customAnswers"
              defaultValue={JSON.stringify(
                profile?.custom_answers ?? {},
                null,
                2,
              )}
            />
          </label>
          <label className="block text-sm font-medium">
            {t("workAuthorization")}
            <p className="mt-1 text-xs font-normal text-muted-foreground">
              {t("workAuthorizationHelp")}
            </p>
            <textarea
              className="mt-2 min-h-24 w-full rounded-md border bg-background p-3 font-mono text-sm"
              name="workAuthorization"
              defaultValue={JSON.stringify(
                profile?.work_authorization ?? [],
                null,
                2,
              )}
            />
          </label>
          <div className="flex gap-3">
            <Button type="submit">{t("saveProfile")}</Button>
            <Button asChild variant="outline">
              <Link href="/applications">{t("cancel")}</Link>
            </Button>
          </div>
        </form>
      </main>
      <MobileNavigation active="applications" />
    </div>
  );
}
