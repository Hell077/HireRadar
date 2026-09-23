import Link from "next/link";

import { Bookmark, BriefcaseBusiness, Clock3, MapPin } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";

import type { Job } from "@/lib/jobs";

export async function JobCard({ job }: { job: Job }) {
  const t = await getTranslations("jobs");

  return (
    <article className="group rounded-xl border bg-card p-5 transition-[border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md sm:p-6">
      <div className="flex items-start gap-4">
        <div className="grid size-11 shrink-0 place-items-center rounded-xl bg-accent text-sm font-bold text-accent-foreground">
          {job.company.slice(0, 2).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div>
              <Link
                href={`/jobs/${job.id}`}
                className="text-lg font-semibold tracking-tight hover:text-primary"
              >
                {job.title}
              </Link>
              <p className="mt-1 text-sm text-muted-foreground">
                {job.company}
              </p>
            </div>
            <Button variant="ghost" size="icon" aria-label={t("saveJob")}>
              <Bookmark aria-hidden="true" />
            </Button>
          </div>

          <div className="mt-4 flex flex-wrap gap-x-4 gap-y-2 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <MapPin className="size-3.5" aria-hidden="true" />
              {job.location}
            </span>
            <span className="flex items-center gap-1.5">
              <BriefcaseBusiness className="size-3.5" aria-hidden="true" />
              {job.employment}
            </span>
            <span className="flex items-center gap-1.5">
              <Clock3 className="size-3.5" aria-hidden="true" />
              {job.posted}
            </span>
          </div>

          <p className="mt-4 text-sm leading-6 text-muted-foreground">
            {job.summary}
          </p>

          <div className="mt-5 flex flex-wrap items-center justify-between gap-4 border-t pt-4">
            <div className="flex flex-wrap gap-2">
              {job.skills.slice(0, 4).map((skill) => (
                <span
                  key={skill}
                  className="rounded-full bg-secondary px-2.5 py-1 text-xs font-medium"
                >
                  {skill}
                </span>
              ))}
            </div>
            <div className="flex items-center gap-3">
              <span className="text-sm font-semibold">{job.salary}</span>
              <span className="rounded-full bg-success-soft px-3 py-1.5 text-xs font-bold text-success">
                {job.match}% {t("match")}
              </span>
            </div>
          </div>
        </div>
      </div>
    </article>
  );
}
