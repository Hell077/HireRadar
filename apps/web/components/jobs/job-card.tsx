"use client";

import { useState } from "react";

import { Bookmark, BriefcaseBusiness, Clock3, MapPin } from "lucide-react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { Button } from "@repo/ui/components/button";

import type { Job } from "@/lib/jobs";

export function JobCard({ job }: { job: Job }) {
  const t = useTranslations("jobs");
  const router = useRouter();
  const [saved, setSaved] = useState(false);
  const href = `/jobs/${job.id}`;

  function openJob() {
    router.push(href);
  }

  return (
    <article
      role="link"
      tabIndex={0}
      aria-label={t("openJob", { title: job.title })}
      onClick={openJob}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          openJob();
        }
      }}
      className={`group cursor-pointer rounded-xl border bg-card p-5 outline-none transition-[border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-ring/40 sm:p-6 ${saved ? "border-primary/50 bg-accent/25" : ""}`}
    >
      <div className="flex items-start gap-4">
        <div className="grid size-11 shrink-0 place-items-center rounded-xl bg-accent text-sm font-bold text-accent-foreground">
          {job.company.slice(0, 2).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold tracking-tight transition-colors group-hover:text-primary">
                {job.title}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {job.company}
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon"
              aria-label={saved ? t("removeSavedJob") : t("saveJob")}
              aria-pressed={saved}
              onClick={(event) => {
                event.stopPropagation();
                setSaved((value) => !value);
              }}
              onKeyDown={(event) => event.stopPropagation()}
              className={saved ? "bg-accent text-primary" : ""}
            >
              <Bookmark
                className={saved ? "fill-current" : ""}
                aria-hidden="true"
              />
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
