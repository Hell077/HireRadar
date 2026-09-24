import Link from "next/link";

import { BookmarkCheck } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";

import { AppHeader } from "@/components/app-header";
import { JobCard } from "@/components/jobs/job-card";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";
import { jobs } from "@/lib/jobs";

export default async function SavedJobsPage() {
  const t = await getTranslations("saved");
  const savedJobs = jobs.slice(0, 2);

  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="saved" />
      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        <Reveal>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="text-sm font-semibold text-primary">
                {t("eyebrow")}
              </p>
              <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">
                {t("title")}
              </h1>
              <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
                {t("description", { count: savedJobs.length })}
              </p>
            </div>
            <Button asChild variant="outline">
              <Link href="/jobs">{t("browse")}</Link>
            </Button>
          </div>
        </Reveal>

        <div className="mt-8 space-y-4">
          {savedJobs.map((job, index) => (
            <Reveal key={job.id} delay={0.06 + index * 0.05}>
              <JobCard job={job} />
            </Reveal>
          ))}
        </div>

        <Reveal delay={0.16}>
          <div className="mt-6 flex items-center gap-3 rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
            <BookmarkCheck
              className="size-5 shrink-0 text-primary"
              aria-hidden="true"
            />
            {t("hint")}
          </div>
        </Reveal>
      </main>
      <MobileNavigation active="saved" />
    </div>
  );
}
