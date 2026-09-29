import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";
import { AppHeader } from "@/components/app-header";
import { MobileNavigation } from "@/components/mobile-navigation";
import { authenticatedRequest } from "@/lib/api/server";
import type { components } from "@/lib/api/generated";
import {
  answerApplicationQuestion,
  cancelApplication,
  retryApplication,
} from "@/app/application-actions";

type Application = components["schemas"]["Application"];
type Question = components["schemas"]["ApplicationQuestion"];

export default async function ApplicationsPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const [response, t, params] = await Promise.all([
    authenticatedRequest("/api/v1/applications", {}, true),
    getTranslations("applications"),
    searchParams,
  ]);
  if (!response) redirect("/sign-in");
  if (response.status === 401) redirect("/sign-in");
  const applications: Application[] = response.ok
    ? ((
        (await response.json()) as components["schemas"]["ApplicationsOutputBody"]
      ).items ?? [])
    : [];
  const [jobsResponse, questionLists] = await Promise.all([
    applications.length
      ? authenticatedRequest(
          `/api/v1/jobs?status=active&ids=${applications.map((a) => encodeURIComponent(a.job_id)).join(",")}&limit=100`,
        )
      : null,
    Promise.all(
      applications
        .filter((a) => a.status === "needs_input")
        .map(async (a) => {
          const r = await authenticatedRequest(
            `/api/v1/applications/${encodeURIComponent(a.id)}/questions`,
          );
          if (!r?.ok) return [a.id, [] as Question[]] as const;
          const body =
            (await r.json()) as components["schemas"]["ApplicationQuestionsOutputBody"];
          return [a.id, body.items ?? []] as const;
        }),
    ),
  ]);
  const titles = new Map<string, string>();
  const applyUrls = new Map<string, string>();
  if (jobsResponse?.ok) {
    const body =
      (await jobsResponse.json()) as components["schemas"]["ListResult"];
    for (const job of body.items ?? []) {
      titles.set(job.id, `${job.title} · ${job.company}`);
      applyUrls.set(job.id, job.apply_url);
    }
  }
  await Promise.all(
    applications
      .filter((a) => a.status === "manual_required" && !applyUrls.has(a.job_id))
      .map(async (application) => {
        const jobResponse = await authenticatedRequest(
          `/api/v1/jobs/${encodeURIComponent(application.job_id)}`,
        );
        if (!jobResponse?.ok) return;
        const job = (await jobResponse.json()) as components["schemas"]["Job"];
        titles.set(job.id, `${job.title} · ${job.company}`);
        applyUrls.set(job.id, job.apply_url);
      }),
  );
  const questions = new Map(questionLists);
  const statuses: Record<string, string> = {
    queued: t("queued"),
    preparing: t("preparing"),
    needs_input: t("needsInput"),
    submitting: t("submitting"),
    submitted: t("submitted"),
    failed: t("failed"),
    manual_required: t("manualRequired"),
    cancelled: t("cancelled"),
  };
  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="applications" />
      <main className="mx-auto w-full max-w-4xl px-4 py-8 sm:px-6 md:py-12">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-3xl font-bold tracking-tight">{t("title")}</h1>
            <p className="mt-2 text-sm text-muted-foreground">
              {t("description")}
            </p>
          </div>
          <Button asChild variant="outline">
            <Link href="/applications/profile">{t("profileLink")}</Link>
          </Button>
        </div>
        {params.error ? (
          <p
            role="alert"
            className="mt-5 rounded-lg bg-destructive-soft p-3 text-sm text-destructive"
          >
            {t("actionError")}
          </p>
        ) : null}
        {applications.length === 0 ? (
          <section className="mt-8 rounded-xl border bg-card p-8 text-center">
            <p className="font-medium">{t("empty")}</p>
            <Link
              className="mt-3 inline-block text-sm text-primary underline"
              href="/jobs"
            >
              {t("browseJobs")}
            </Link>
          </section>
        ) : (
          <div className="mt-8 space-y-4">
            {applications.map((application) => (
              <article
                key={application.id}
                className="rounded-xl border bg-card p-5 sm:p-6"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h2 className="font-semibold">
                      {titles.get(application.job_id) ?? t("jobUnavailable")}
                    </h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {application.provider} ·{" "}
                      {t("attempts", { count: application.attempts })}
                    </p>
                  </div>
                  <span className="rounded-full bg-accent px-3 py-1 text-xs font-semibold">
                    {statuses[application.status] ?? application.status}
                  </span>
                </div>
                {application.status === "manual_required" &&
                applyUrls.has(application.job_id) ? (
                  <p className="mt-4 text-sm text-muted-foreground">
                    {t("manualRequiredHelp")}{" "}
                    <a
                      className="text-primary underline"
                      href={applyUrls.get(application.job_id)}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {t("openEmployerSite")}
                    </a>
                  </p>
                ) : null}
                {application.submitted_at ? (
                  <p className="mt-3 text-sm text-muted-foreground">
                    {t("submittedAt", {
                      date: new Date(application.submitted_at).toLocaleString(),
                    })}
                  </p>
                ) : null}
                {application.status === "needs_input" ? (
                  <div className="mt-5 space-y-4">
                    {(questions.get(application.id) ?? [])
                      .filter((q) => q.status !== "answered")
                      .map((question) => (
                        <form
                          action={answerApplicationQuestion}
                          key={question.id}
                          className="rounded-lg border p-4"
                        >
                          <input
                            type="hidden"
                            name="questionId"
                            value={question.id}
                          />
                          <label className="block text-sm font-medium">
                            {question.question}
                            {question.required ? " *" : ""}
                            {question.options?.length ? (
                              <select
                                className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                                name="answer"
                                required={question.required}
                                defaultValue=""
                              >
                                <option value="" disabled>
                                  {t("chooseAnswer")}
                                </option>
                                {question.options.map((o, i) => (
                                  <option key={i} value={String(o.value)}>
                                    {o.label}
                                  </option>
                                ))}
                              </select>
                            ) : (
                              <input
                                className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                                name="answer"
                                required={question.required}
                                defaultValue={
                                  typeof question.answer === "string"
                                    ? question.answer
                                    : ""
                                }
                              />
                            )}
                          </label>
                          <Button className="mt-3" size="sm" type="submit">
                            {t("sendAnswer")}
                          </Button>
                        </form>
                      ))}
                  </div>
                ) : null}
                <div className="mt-4 flex flex-wrap gap-2">
                  {application.status === "failed" ? (
                    <form action={retryApplication}>
                      <input
                        type="hidden"
                        name="applicationId"
                        value={application.id}
                      />
                      <Button variant="outline" size="sm" type="submit">
                        {t("retry")}
                      </Button>
                    </form>
                  ) : null}
                  {["queued", "preparing", "needs_input"].includes(
                    application.status,
                  ) ? (
                    <form action={cancelApplication}>
                      <input
                        type="hidden"
                        name="applicationId"
                        value={application.id}
                      />
                      <Button variant="ghost" size="sm" type="submit">
                        {t("cancel")}
                      </Button>
                    </form>
                  ) : null}
                </div>
              </article>
            ))}
          </div>
        )}
      </main>
      <MobileNavigation active="applications" />
    </div>
  );
}
