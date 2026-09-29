import Link from "next/link";
import {
  BellRing,
  Check,
  FileText,
  Link as LinkIcon,
  MapPin,
  Send,
} from "lucide-react";
import { getLocale, getTranslations } from "next-intl/server";
import { redirect } from "next/navigation";

import { Button } from "@repo/ui/components/button";

import { AppHeader } from "@/components/app-header";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";
import {
  EditProfileDialog,
  EditSkillsSheet,
  ReplaceResumeDialog,
} from "@/components/profile/profile-editors";
import { getCandidateData } from "@/lib/api/server";
import { connectTelegram } from "@/app/settings-actions";
import { deleteResume, downloadResume } from "@/app/resume-actions";
import { normalizePositions } from "@/lib/profile/positions";

function SectionHeading({
  title,
  action,
}: {
  title: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4">
      <h2 className="text-[17px] font-semibold tracking-tight">{title}</h2>
      {action ?? null}
    </div>
  );
}

export default async function ProfilePage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const t = await getTranslations("profile");
  const locale = await getLocale();
  const status = await searchParams;
  const candidate = await getCandidateData();
  if (!candidate) redirect("/sign-in");
  const {
    profile,
    skills,
    positions,
    preferences,
    resumes,
    sources,
    telegram,
  } = candidate;
  const normalizedPositions = normalizePositions(positions);
  const latestResume = [...resumes].sort(
    (a, b) => Date.parse(b.created_at) - Date.parse(a.created_at),
  )[0];
  const enabledSources = sources.filter(({ enabled }) => enabled);
  const fullName =
    [profile.first_name, profile.last_name].filter(Boolean).join(" ") ||
    t("unnamed");
  const initials =
    [profile.first_name, profile.last_name]
      .filter(Boolean)
      .map((part) => part[0])
      .join("")
      .toUpperCase() || "?";
  const countryName = profile.country
    ? new Intl.DisplayNames([locale], { type: "region" }).of(profile.country)
    : "";
  const location =
    [profile.city, countryName].filter(Boolean).join(", ") ||
    t("locationMissing");
  const missingFields = [
    profile.first_name ? null : t("missingFirstName"),
    profile.last_name ? null : t("missingLastName"),
    profile.country ? null : t("missingCountry"),
    profile.city ? null : t("missingCity"),
    profile.seniority ? null : t("missingSeniority"),
    profile.desired_salary?.amount ? null : t("missingSalary"),
    skills.length > 0 ? null : t("missingSkills"),
  ].filter((field): field is string => Boolean(field));
  const completeness = Math.round(((7 - missingFields.length) / 7) * 100);
  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader />

      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        {status.profileUpdated === "1" || status.skillsUpdated === "1" ? (
          <p className="mb-6 rounded-lg bg-success-soft p-3 text-sm text-success">
            {t("saved")}
          </p>
        ) : status.profileError === "1" || status.skillsError === "1" ? (
          <p className="mb-6 rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t("saveError")}
          </p>
        ) : null}
        <Reveal>
          <section className="border-border flex flex-col items-center gap-5 border-b pb-8 text-center sm:flex-row sm:text-left">
            <div className="bg-accent text-accent-foreground grid size-19 shrink-0 place-items-center rounded-3xl text-xl font-bold">
              {initials}
            </div>
            <div className="min-w-0 flex-1">
              <h1 className="text-[27px] font-bold tracking-tight">
                {fullName}
              </h1>
              <p className="text-muted-foreground mt-1 text-[15px] font-medium">
                {profile.seniority
                  ? t("seniorityRole", { seniority: profile.seniority })
                  : t("roleMissing")}
              </p>
              <p className="text-muted-foreground mt-2 flex items-center justify-center gap-1.5 text-xs sm:justify-start">
                <MapPin className="size-3.5" aria-hidden="true" />
                {location}
              </p>
            </div>
            <EditProfileDialog profile={profile} />
          </section>
        </Reveal>

        <Reveal
          className="mt-8 grid gap-6 md:grid-cols-[minmax(0,1fr)_320px]"
          delay={0.08}
          distance={18}
        >
          <div className="space-y-6">
            <section className="border-border bg-card rounded-xl border p-5 sm:p-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <SectionHeading title={t("resume")} />
                  <p className="text-muted-foreground mt-1 text-xs">
                    {t("resumeHelp")}
                  </p>
                </div>
                <ReplaceResumeDialog />
              </div>

              <div className="bg-secondary mt-4 flex items-center gap-3 rounded-lg p-4">
                <span className="bg-card text-primary grid size-9.5 shrink-0 place-items-center rounded-[10px]">
                  <FileText className="size-4.5" aria-hidden="true" />
                </span>
                <div className="min-w-0">
                  <p className="truncate text-[13px] font-semibold">
                    {latestResume?.file_name ?? t("noResume")}
                  </p>
                  <p className="text-muted-foreground mt-0.5 text-[11px]">
                    {latestResume
                      ? `${latestResume.status} · ${new Date(latestResume.updated_at).toLocaleDateString()}`
                      : t("resumeMissing")}
                  </p>
                </div>
                {latestResume ? (
                  <div className="ml-auto flex gap-2">
                    <form action={downloadResume}>
                      <input
                        type="hidden"
                        name="resumeId"
                        value={latestResume.id}
                      />
                      <Button type="submit" size="sm" variant="outline">
                        {t("downloadResume")}
                      </Button>
                    </form>
                    <form action={deleteResume}>
                      <input
                        type="hidden"
                        name="resumeId"
                        value={latestResume.id}
                      />
                      <Button type="submit" size="sm" variant="ghost">
                        {t("deleteResume")}
                      </Button>
                    </form>
                  </div>
                ) : null}
              </div>
            </section>

            <section className="border-border bg-card rounded-xl border p-5 sm:p-6">
              <SectionHeading
                title={t("skills")}
                action={<EditSkillsSheet initialSkills={skills} />}
              />
              <div className="mt-4 flex flex-wrap gap-2">
                {skills.map((skill) => (
                  <span
                    key={skill.name}
                    className="bg-secondary rounded-full px-3 py-2 text-xs font-semibold"
                  >
                    {skill.name}
                  </span>
                ))}
              </div>
            </section>

            <section className="border-border bg-card rounded-xl border p-5 sm:p-6">
              <SectionHeading
                title={t("jobPreferences")}
                action={
                  <Button
                    asChild
                    variant="ghost"
                    size="sm"
                    className="text-primary"
                  >
                    <Link href="/settings#job-preferences">{t("edit")}</Link>
                  </Button>
                }
              />
              <dl className="mt-5 grid gap-5 sm:grid-cols-3">
                {[
                  [t("roles"), normalizedPositions.join(" · ") || "—"],
                  [
                    t("availability"),
                    t("availabilityFrom", {
                      country: countryName || profile.country || "—",
                    }),
                  ],
                  [
                    t("employment"),
                    [
                      preferences?.employment_types?.includes("full_time")
                        ? t("employmentFullTime")
                        : null,
                      preferences?.employment_types?.includes("part_time")
                        ? t("employmentPartTime")
                        : null,
                      preferences?.employment_types?.some((value) =>
                        ["contract", "b2b", "freelance"].includes(value),
                      )
                        ? t("employmentProject")
                        : null,
                    ]
                      .filter(Boolean)
                      .join(" · ") || "—",
                  ],
                ].map(([label, value]) => (
                  <div key={label}>
                    <dt className="text-muted-foreground text-[11px] font-semibold">
                      {label}
                    </dt>
                    <dd className="mt-1 text-[13px] font-semibold leading-5">
                      {value}
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          </div>

          <aside className="space-y-4">
            <section className="border-border bg-card rounded-xl border p-5">
              <div className="flex items-center justify-between text-sm font-semibold">
                <span>{t("completeness")}</span>
                <span className="text-primary">{completeness}%</span>
              </div>
              <div className="bg-secondary mt-3 h-1.5 overflow-hidden rounded-full">
                <div
                  className="bg-primary h-full rounded-full"
                  style={{ width: `${completeness}%` }}
                />
              </div>
              {missingFields.length ? (
                <p className="text-muted-foreground mt-3 text-xs leading-5">
                  {t("incompleteHelp", { fields: missingFields.join(" · ") })}
                </p>
              ) : null}
            </section>

            <section className="border-border bg-card rounded-xl border p-5">
              <div className="flex items-center gap-3">
                <span className="bg-accent text-accent-foreground grid size-9 shrink-0 place-items-center rounded-[11px]">
                  <Send className="size-4" aria-hidden="true" />
                </span>
                <div>
                  <h2 className="text-sm font-semibold">{t("telegram")}</h2>
                  <p className="text-muted-foreground text-[11px]">
                    {telegram?.connected && telegram.enabled !== false
                      ? `@${telegram.username ?? "telegram"}`
                      : telegram?.connected
                        ? t("telegramReconnectRequired")
                        : t("notConnected")}
                  </p>
                </div>
              </div>
              <p className="text-muted-foreground mt-4 text-xs leading-5">
                {t("telegramHelp")}
              </p>
              {!telegram?.connected || telegram.enabled === false ? (
                <form action={connectTelegram}>
                  <Button className="mt-4 w-full" size="lg" type="submit">
                    <LinkIcon aria-hidden="true" />
                    {telegram?.connected
                      ? t("reconnectTelegram")
                      : t("connectTelegram")}
                  </Button>
                </form>
              ) : null}
            </section>

            <section className="border-border bg-card rounded-xl border p-5">
              <div className="flex items-center gap-2">
                <BellRing className="text-primary size-4" aria-hidden="true" />
                <h2 className="text-sm font-semibold">{t("sources")}</h2>
              </div>
              <p className="mt-3 text-[13px] font-semibold">
                {t("sourcesEnabledCount", { count: enabledSources.length })}
              </p>
              <p className="text-muted-foreground mt-1 text-[11px] leading-4">
                {enabledSources.map(({ source_id }) => source_id).join(", ") ||
                  "—"}
              </p>
            </section>

            <div className="bg-success-soft text-success flex items-center gap-2 rounded-lg px-4 py-3 text-xs font-semibold">
              <Check className="size-4" aria-hidden="true" />
              {t("confirmed")}
            </div>
          </aside>
        </Reveal>
      </main>

      <MobileNavigation />
    </div>
  );
}
