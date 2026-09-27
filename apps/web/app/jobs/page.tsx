import { BriefcaseBusiness, SlidersHorizontal } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";

import { AppHeader } from "@/components/app-header";
import { JobCard } from "@/components/jobs/job-card";
import { StatusState } from "@/components/feedback/status-state";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";
import { getJobs } from "@/lib/jobs";

export default async function JobsPage({
  searchParams,
}: {
  searchParams: Promise<{
    empty?: string;
    remote_policy?: string;
    country?: string;
    eligibility?: string;
  }>;
}) {
  const t = await getTranslations("jobs");
  const params = await searchParams;
  const query = new URLSearchParams({ status: "active", limit: "100" });
  if (params.remote_policy) query.set("remote_policy", params.remote_policy);
  if (params.country) query.set("country", params.country.toUpperCase());
  if (params.eligibility) query.set("eligibility", params.eligibility);
  const jobs = await getJobs(query.toString());
  const empty = params.empty === "1" || jobs.length === 0;

  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="jobs" />
      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        <Reveal>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-sm font-semibold text-primary">
                {t("eyebrow")}
              </p>
              <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">
                {t("title")}
              </h1>
              <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
                {t("description")}
              </p>
            </div>
            <p className="text-sm text-muted-foreground">{t("updated")}</p>
          </div>
        </Reveal>

        <Reveal delay={0.06}>
          <section
            className="mt-8 rounded-xl border bg-card p-4"
            aria-label={t("filters")}
          >
            <form className="grid gap-3 sm:grid-cols-[1fr_1fr_1fr_auto]">
              <select
                name="remote_policy"
                defaultValue={params.remote_policy ?? ""}
                className="h-11 rounded-md border bg-background px-3 text-sm"
              >
                <option value="">{t("allRemotePolicies")}</option>
                <option value="worldwide">Worldwide</option>
                <option value="remote">Remote</option>
                <option value="remote_region">Remote region</option>
              </select>
              <input
                name="country"
                defaultValue={params.country ?? ""}
                maxLength={2}
                placeholder={t("countryCode")}
                className="h-11 rounded-md border bg-background px-3 text-sm uppercase"
              />
              <select
                name="eligibility"
                defaultValue={params.eligibility ?? ""}
                className="h-11 rounded-md border bg-background px-3 text-sm"
              >
                <option value="">{t("allEligibility")}</option>
                <option value="eligible">Eligible</option>
                <option value="unknown">Unknown</option>
              </select>
              <Button type="submit" variant="outline" size="lg">
                <SlidersHorizontal aria-hidden="true" />
                {t("filters")}
              </Button>
            </form>
          </section>
        </Reveal>

        <div className="mt-6 space-y-4">
          {empty ? (
            <StatusState
              icon={<BriefcaseBusiness className="size-5" aria-hidden="true" />}
              title={t("emptyTitle")}
              description={t("emptyDescription")}
              actionHref="/settings"
              actionLabel={t("adjustSettings")}
            />
          ) : (
            jobs.map((job, index) => (
              <Reveal key={job.id} delay={0.08 + index * 0.05}>
                <JobCard job={job} />
              </Reveal>
            ))
          )}
        </div>
      </main>
      <MobileNavigation active="jobs" />
    </div>
  );
}
