import {
  BellRing,
  Check,
  FileText,
  Link as LinkIcon,
  MapPin,
  Pencil,
  Send,
  Upload,
} from "lucide-react";
import { getTranslations } from "next-intl/server";

import { Button } from "@repo/ui/components/button";

import { AppHeader } from "@/components/app-header";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";

const skills = ["Go", "React", "Next.js", "TypeScript", "PostgreSQL", "Docker"];

function SectionHeading({ title, action }: { title: string; action?: string }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <h2 className="text-[17px] font-semibold tracking-tight">{title}</h2>
      {action ? (
        <button
          type="button"
          className="text-primary flex items-center gap-1.5 text-xs font-semibold"
        >
          <Pencil className="size-3.5" aria-hidden="true" />
          {action}
        </button>
      ) : null}
    </div>
  );
}

export default async function ProfilePage() {
  const t = await getTranslations("profile");
  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader />

      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        <Reveal>
          <section className="border-border flex flex-col items-center gap-5 border-b pb-8 text-center sm:flex-row sm:text-left">
            <div className="bg-accent text-accent-foreground grid size-19 shrink-0 place-items-center rounded-3xl text-xl font-bold">
              TK
            </div>
            <div className="min-w-0 flex-1">
              <h1 className="text-[27px] font-bold tracking-tight">Timur K.</h1>
              <p className="text-muted-foreground mt-1 text-[15px] font-medium">
                {t("role")}
              </p>
              <p className="text-muted-foreground mt-2 flex items-center justify-center gap-1.5 text-xs sm:justify-start">
                <MapPin className="size-3.5" aria-hidden="true" />
                {t("location")}
              </p>
            </div>
            <Button variant="outline" size="lg">
              <Pencil aria-hidden="true" />
              {t("editProfile")}
            </Button>
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
                <Button variant="outline" size="sm">
                  <Upload aria-hidden="true" />
                  {t("replace")}
                </Button>
              </div>

              <div className="bg-secondary mt-4 flex items-center gap-3 rounded-lg p-4">
                <span className="bg-card text-primary grid size-9.5 shrink-0 place-items-center rounded-[10px]">
                  <FileText className="size-4.5" aria-hidden="true" />
                </span>
                <div className="min-w-0">
                  <p className="truncate text-[13px] font-semibold">
                    timur-k-resume.pdf
                  </p>
                  <p className="text-muted-foreground mt-0.5 text-[11px]">
                    {t("resumeStatus")}
                  </p>
                </div>
              </div>
            </section>

            <section className="border-border bg-card rounded-xl border p-5 sm:p-6">
              <SectionHeading title={t("skills")} action={t("editProfile")} />
              <div className="mt-4 flex flex-wrap gap-2">
                {skills.map((skill) => (
                  <span
                    key={skill}
                    className="bg-secondary rounded-full px-3 py-2 text-xs font-semibold"
                  >
                    {skill}
                  </span>
                ))}
              </div>
            </section>

            <section className="border-border bg-card rounded-xl border p-5 sm:p-6">
              <SectionHeading
                title={t("jobPreferences")}
                action={t("editProfile")}
              />
              <dl className="mt-5 grid gap-5 sm:grid-cols-3">
                {[
                  [t("roles"), t("rolesValue")],
                  [t("remoteRegion"), t("regionValue")],
                  [t("employment"), t("employmentValue")],
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
                <span className="text-primary">82%</span>
              </div>
              <div className="bg-secondary mt-3 h-1.5 overflow-hidden rounded-full">
                <div className="bg-primary h-full w-[82%] rounded-full" />
              </div>
              <p className="text-muted-foreground mt-3 text-xs leading-5">
                {t("completenessHelp")}
              </p>
            </section>

            <section className="border-border bg-card rounded-xl border p-5">
              <div className="flex items-center gap-3">
                <span className="bg-accent text-accent-foreground grid size-9 shrink-0 place-items-center rounded-[11px]">
                  <Send className="size-4" aria-hidden="true" />
                </span>
                <div>
                  <h2 className="text-sm font-semibold">{t("telegram")}</h2>
                  <p className="text-muted-foreground text-[11px]">
                    {t("notConnected")}
                  </p>
                </div>
              </div>
              <p className="text-muted-foreground mt-4 text-xs leading-5">
                {t("telegramHelp")}
              </p>
              <Button className="mt-4 w-full" size="lg">
                <LinkIcon aria-hidden="true" />
                {t("connectTelegram")}
              </Button>
            </section>

            <section className="border-border bg-card rounded-xl border p-5">
              <div className="flex items-center gap-2">
                <BellRing className="text-primary size-4" aria-hidden="true" />
                <h2 className="text-sm font-semibold">{t("sources")}</h2>
              </div>
              <p className="mt-3 text-[13px] font-semibold">
                {t("sourcesEnabled")}
              </p>
              <p className="text-muted-foreground mt-1 text-[11px] leading-4">
                {t("sourcesList")}
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
