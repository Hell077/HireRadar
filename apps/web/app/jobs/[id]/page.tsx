import Link from "next/link";
import { notFound } from "next/navigation";

import {
  ArrowLeft,
  Bookmark,
  BriefcaseBusiness,
  Check,
  ExternalLink,
  MapPin,
  X,
} from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";

import { AppHeader } from "@/components/app-header";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";
import { getJob, jobs } from "@/lib/jobs";

export function generateStaticParams() {
  return jobs.map((job) => ({ id: job.id }));
}

export default async function JobDetailsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const job = getJob(id);
  if (!job) notFound();
  const t = await getTranslations("jobs");

  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="jobs" />
      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        <Reveal>
          <Link
            href="/jobs"
            className="inline-flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("backToJobs")}
          </Link>
          <section className="mt-6 rounded-xl border bg-card p-5 sm:p-8">
            <div className="flex flex-col gap-5 sm:flex-row sm:items-start">
              <div className="grid size-14 shrink-0 place-items-center rounded-2xl bg-accent text-lg font-bold text-accent-foreground">
                {job.company.slice(0, 2).toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
                      {job.title}
                    </h1>
                    <p className="mt-2 text-base text-muted-foreground">
                      {job.company}
                    </p>
                  </div>
                  <span className="w-fit rounded-full bg-success-soft px-4 py-2 text-sm font-bold text-success">
                    {job.match}% {t("match")}
                  </span>
                </div>
                <div className="mt-5 flex flex-wrap gap-x-5 gap-y-2 text-sm text-muted-foreground">
                  <span className="flex items-center gap-1.5">
                    <MapPin className="size-4" aria-hidden="true" />
                    {job.location}
                  </span>
                  <span className="flex items-center gap-1.5">
                    <BriefcaseBusiness className="size-4" aria-hidden="true" />
                    {job.employment}
                  </span>
                  <span>{job.salary}</span>
                  <span>{t("via", { source: job.source })}</span>
                </div>
                <div className="mt-6 flex flex-wrap gap-3">
                  <Button size="lg">
                    {t("apply")}
                    <ExternalLink aria-hidden="true" />
                  </Button>
                  <Button variant="outline" size="lg">
                    <Bookmark aria-hidden="true" />
                    {t("save")}
                  </Button>
                </div>
              </div>
            </div>
          </section>
        </Reveal>

        <div className="mt-6 grid gap-6 md:grid-cols-[minmax(0,1fr)_300px]">
          <Reveal delay={0.08}>
            <article className="space-y-8 rounded-xl border bg-card p-5 sm:p-8">
              <section>
                <h2 className="text-lg font-semibold">{t("aboutRole")}</h2>
                <p className="mt-3 text-sm leading-7 text-muted-foreground">
                  {job.summary} {t("descriptionBody")}
                </p>
              </section>
              <section>
                <h2 className="text-lg font-semibold">
                  {t("responsibilities")}
                </h2>
                <ul className="mt-3 space-y-3 text-sm leading-6 text-muted-foreground">
                  {[
                    t("responsibility1"),
                    t("responsibility2"),
                    t("responsibility3"),
                  ].map((item) => (
                    <li key={item} className="flex gap-3">
                      <Check
                        className="mt-1 size-4 shrink-0 text-success"
                        aria-hidden="true"
                      />
                      {item}
                    </li>
                  ))}
                </ul>
              </section>
              <section>
                <h2 className="text-lg font-semibold">{t("requirements")}</h2>
                <div className="mt-3 flex flex-wrap gap-2">
                  {job.skills.map((skill) => (
                    <span
                      key={skill}
                      className="rounded-full bg-secondary px-3 py-1.5 text-xs font-medium"
                    >
                      {skill}
                    </span>
                  ))}
                </div>
              </section>
            </article>
          </Reveal>
          <aside className="space-y-4">
            <Reveal delay={0.12}>
              <section className="rounded-xl border bg-card p-5">
                <h2 className="font-semibold">{t("whyMatch")}</h2>
                <p className="mt-2 text-xs leading-5 text-muted-foreground">
                  {t("matchExplanation")}
                </p>
                <div className="mt-4 space-y-2">
                  {job.skills.map((skill) => (
                    <div
                      key={skill}
                      className="flex items-center gap-2 text-sm"
                    >
                      <Check
                        className="size-4 text-success"
                        aria-hidden="true"
                      />
                      {skill}
                    </div>
                  ))}
                </div>
              </section>
            </Reveal>
            <Reveal delay={0.16}>
              <section className="rounded-xl border bg-card p-5">
                <h2 className="font-semibold">{t("missingSkills")}</h2>
                {job.missingSkills.map((skill) => (
                  <div
                    key={skill}
                    className="mt-3 flex items-center gap-2 text-sm text-muted-foreground"
                  >
                    <X className="size-4 text-warning" aria-hidden="true" />
                    {skill}
                  </div>
                ))}
                <p className="mt-3 text-xs leading-5 text-muted-foreground">
                  {t("missingHelp")}
                </p>
              </section>
            </Reveal>
          </aside>
        </div>
      </main>
      <MobileNavigation active="jobs" />
    </div>
  );
}
