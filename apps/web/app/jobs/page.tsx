import { BriefcaseBusiness, Search, SlidersHorizontal } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";

import { AppHeader } from "@/components/app-header";
import { JobCard } from "@/components/jobs/job-card";
import { StatusState } from "@/components/feedback/status-state";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";
import { jobs } from "@/lib/jobs";

export default async function JobsPage({
  searchParams,
}: {
  searchParams: Promise<{ empty?: string }>;
}) {
  const t = await getTranslations("jobs");
  const empty = (await searchParams).empty === "1";

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
            <div className="flex flex-col gap-3 sm:flex-row">
              <div className="relative flex-1">
                <Search
                  className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
                  aria-hidden="true"
                />
                <Input
                  className="h-11 pl-9"
                  placeholder={t("searchPlaceholder")}
                />
              </div>
              <Button variant="outline" size="lg">
                <SlidersHorizontal aria-hidden="true" />
                {t("filters")}
              </Button>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {[
                t("allMatches"),
                "Go",
                "React",
                t("worldwide"),
                t("contract"),
              ].map((filter, index) => (
                <button
                  key={filter}
                  type="button"
                  className={
                    index === 0
                      ? "rounded-full bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground"
                      : "rounded-full bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground hover:bg-accent"
                  }
                >
                  {filter}
                </button>
              ))}
            </div>
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
