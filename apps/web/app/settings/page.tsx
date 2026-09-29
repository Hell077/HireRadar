import { BellRing, Database, Send, SlidersHorizontal } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { Button } from "@repo/ui/components/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { AppHeader } from "@/components/app-header";
import { MobileNavigation } from "@/components/mobile-navigation";
import { Reveal } from "@/components/motion/reveal";
import { PositionSelector } from "@/components/preferences/position-selector";
import {
  connectTelegram,
  disconnectTelegram,
  saveSettings,
} from "@/app/settings-actions";
import { getSettingsData } from "@/lib/settings";

function Check({
  name,
  value,
  label,
  checked,
  description,
}: {
  name: string;
  value?: string;
  label: string;
  checked: boolean;
  description?: string;
}) {
  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-lg border p-3 text-sm">
      <input
        className="size-4 accent-primary"
        type="checkbox"
        name={name}
        value={value}
        defaultChecked={checked}
      />
      <span className="min-w-0">
        <span className="block font-medium">{label}</span>
        {description ? (
          <span className="mt-1 block text-xs leading-5 text-muted-foreground">
            {description}
          </span>
        ) : null}
      </span>
    </label>
  );
}

export default async function SettingsPage({
  searchParams,
}: {
  searchParams: Promise<{
    saved?: string;
    error?: string;
    telegramError?: string;
  }>;
}) {
  const t = await getTranslations("settings");
  const params = await searchParams;
  const data = await getSettingsData();
  const enabledSources = new Set(
    data.sourcePreferences
      .filter(({ enabled }) => enabled)
      .map(({ source_id }) => source_id),
  );
  const preferences = data.preferences;
  const notifications = data.notifications;

  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="settings" />
      <main className="mx-auto w-full max-w-4xl px-4 py-8 sm:px-6 md:py-12">
        <Reveal>
          <p className="text-sm font-semibold text-primary">{t("eyebrow")}</p>
          <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">
            {t("title")}
          </h1>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
            {t("description")}
          </p>
        </Reveal>

        {params.saved === "1" ? (
          <p className="mt-6 rounded-lg bg-success-soft p-3 text-sm text-success">
            {t("saved")}
          </p>
        ) : null}
        {params.error === "1" || params.telegramError === "1" ? (
          <p className="mt-6 rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t("saveError")}
          </p>
        ) : null}

        <form action={saveSettings} className="mt-8 space-y-6">
          <Reveal delay={0.05}>
            <Card>
              <CardHeader className="border-b">
                <CardTitle className="flex items-center gap-2">
                  <Database className="size-5 text-primary" />
                  {t("sources")}
                </CardTitle>
                <p className="text-sm text-muted-foreground">
                  {t("sourcesHelp")}
                </p>
              </CardHeader>
              <CardContent className="grid gap-3 sm:grid-cols-2">
                {data.sources.length === 0 ? (
                  <p className="text-sm text-muted-foreground sm:col-span-2">
                    {t("sourcesEmpty")}
                  </p>
                ) : (
                  data.sources.map((source) => (
                    <div key={source.id}>
                      <input type="hidden" name="allSource" value={source.id} />
                      <Check
                        name="source"
                        value={source.id}
                        label={`${source.name} · ${source.type}`}
                        checked={enabledSources.has(source.id)}
                      />
                    </div>
                  ))
                )}
              </CardContent>
            </Card>
          </Reveal>

          <Reveal delay={0.1}>
            <Card id="job-preferences" className="scroll-mt-24">
              <CardHeader className="border-b">
                <CardTitle className="flex items-center gap-2">
                  <SlidersHorizontal className="size-5 text-primary" />
                  {t("jobPreferences")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-5">
                <PositionSelector initialPositions={data.positions} />
                {["worldwide", "remote", "remote_region", "remote_country"].map(
                  (value) => (
                    <input
                      key={value}
                      type="hidden"
                      name="remotePolicy"
                      value={value}
                    />
                  ),
                )}
                <p className="rounded-lg bg-secondary/50 p-3 text-sm text-muted-foreground">
                  {t("eligibilityAutomatic")}
                </p>
                <div className="grid gap-3 sm:grid-cols-3">
                  {(
                    [
                      ["full_time", t("workFullTime")],
                      ["part_time", t("workPartTime")],
                      ["project", t("workProject")],
                    ] as const
                  ).map(([value, label]) => (
                    <Check
                      key={value}
                      name="employmentType"
                      value={value}
                      label={label}
                      description={
                        value === "full_time"
                          ? t("workFullTimeHelp")
                          : value === "part_time"
                            ? t("workPartTimeHelp")
                            : t("workProjectHelp")
                      }
                      checked={
                        value === "project"
                          ? (preferences?.employment_types?.some((type) =>
                              ["contract", "b2b", "freelance"].includes(type),
                            ) ?? true)
                          : (preferences?.employment_types?.includes(value) ??
                            value === "part_time")
                      }
                    />
                  ))}
                </div>
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="text-sm font-medium">
                    {t("minimumMatch")}
                    <input
                      className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                      type="number"
                      min="0"
                      max="100"
                      name="minimumScore"
                      defaultValue={preferences?.minimum_match_score ?? 70}
                    />
                  </label>
                  <label className="text-sm font-medium">
                    {t("maximumJobAge")}
                    <input
                      className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                      type="number"
                      min="1"
                      max="365"
                      name="maximumJobAge"
                      defaultValue={preferences?.maximum_job_age_days ?? 30}
                    />
                  </label>
                </div>
              </CardContent>
            </Card>
          </Reveal>

          <Reveal delay={0.15}>
            <Card>
              <CardHeader className="border-b">
                <CardTitle className="flex items-center gap-2">
                  <BellRing className="size-5 text-primary" />
                  {t("notifications")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-3 sm:grid-cols-3">
                  <Check
                    name="notificationsEnabled"
                    label={t("telegramMatches")}
                    checked={
                      notifications?.enabled ??
                      preferences?.notifications_enabled ??
                      true
                    }
                  />
                  <Check
                    name="immediate"
                    label={t("instant")}
                    checked={notifications?.immediate ?? true}
                  />
                  <Check
                    name="digestEnabled"
                    label={t("daily")}
                    checked={notifications?.digest_enabled ?? false}
                  />
                </div>
                <div className="grid gap-4 sm:grid-cols-2">
                  <label className="text-sm font-medium">
                    {t("timezone")}
                    <input
                      className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                      name="timezone"
                      defaultValue={notifications?.timezone ?? "UTC"}
                    />
                  </label>
                  <label className="text-sm font-medium">
                    {t("maxPerDay")}
                    <input
                      className="mt-2 h-11 w-full rounded-md border bg-background px-3"
                      type="number"
                      min="1"
                      max="100"
                      name="maxPerDay"
                      defaultValue={notifications?.max_per_day ?? 10}
                    />
                  </label>
                </div>
              </CardContent>
            </Card>
          </Reveal>
          <div className="flex justify-end">
            <Button type="submit" size="lg">
              {t("save")}
            </Button>
          </div>
        </form>

        <Reveal delay={0.2} className="mt-6">
          <Card>
            <CardContent className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <span className="grid size-10 place-items-center rounded-xl bg-accent text-primary">
                  <Send className="size-4" />
                </span>
                <div>
                  <p className="text-sm font-semibold">
                    {data.telegram?.connected
                      ? t("telegramConnected")
                      : t("telegramNotConnected")}
                  </p>
                  {data.telegram?.username ? (
                    <p className="text-xs text-muted-foreground">
                      @{data.telegram.username}
                    </p>
                  ) : null}
                </div>
              </div>
              <form
                action={
                  data.telegram?.connected
                    ? disconnectTelegram
                    : connectTelegram
                }
              >
                <Button type="submit" variant="outline">
                  {data.telegram?.connected ? t("disconnect") : t("connect")}
                </Button>
              </form>
            </CardContent>
          </Card>
        </Reveal>
      </main>
      <MobileNavigation active="settings" />
    </div>
  );
}
