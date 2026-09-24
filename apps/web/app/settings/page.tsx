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
import { SettingToggle } from "@/components/settings/setting-toggle";
import { SaveSettingsButton } from "@/components/settings/save-settings-button";

const sourceKeys = [
  "greenhouse",
  "lever",
  "ashby",
  "remotive",
  "remoteok",
  "wellfound",
] as const;

export default async function SettingsPage() {
  const t = await getTranslations("settings");

  return (
    <div className="min-h-screen pb-24 md:pb-0">
      <AppHeader active="settings" />
      <main className="mx-auto w-full max-w-4xl px-4 py-8 sm:px-6 md:py-12">
        <Reveal>
          <div>
            <p className="text-sm font-semibold text-primary">{t("eyebrow")}</p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight sm:text-4xl">
              {t("title")}
            </h1>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
              {t("description")}
            </p>
          </div>
        </Reveal>

        <div className="mt-8 space-y-6">
          <Reveal delay={0.05}>
            <Card>
              <CardHeader className="border-b">
                <CardTitle className="flex items-center gap-2">
                  <Database
                    className="size-5 text-primary"
                    aria-hidden="true"
                  />
                  {t("sources")}
                </CardTitle>
                <p className="text-sm leading-6 text-muted-foreground">
                  {t("sourcesHelp")}
                </p>
              </CardHeader>
              <CardContent className="divide-y">
                {sourceKeys.map((source, index) => (
                  <SettingToggle
                    key={source}
                    label={t(`source.${source}`)}
                    description={t(`sourceHelp.${source}`)}
                    defaultChecked={index < 4}
                  />
                ))}
              </CardContent>
            </Card>
          </Reveal>

          <Reveal delay={0.1}>
            <Card>
              <CardHeader className="border-b">
                <CardTitle className="flex items-center gap-2">
                  <BellRing
                    className="size-5 text-primary"
                    aria-hidden="true"
                  />
                  {t("notifications")}
                </CardTitle>
              </CardHeader>
              <CardContent className="divide-y">
                <SettingToggle
                  label={t("telegramMatches")}
                  description={t("telegramMatchesHelp")}
                  defaultChecked
                />
                <SettingToggle
                  label={t("emailDigest")}
                  description={t("emailDigestHelp")}
                />
                <SettingToggle
                  label={t("highMatchOnly")}
                  description={t("highMatchOnlyHelp")}
                  defaultChecked
                />
              </CardContent>
            </Card>
          </Reveal>

          <Reveal delay={0.15}>
            <Card>
              <CardHeader className="border-b">
                <CardTitle className="flex items-center gap-2">
                  <SlidersHorizontal
                    className="size-5 text-primary"
                    aria-hidden="true"
                  />
                  {t("delivery")}
                </CardTitle>
              </CardHeader>
              <CardContent className="grid gap-5 sm:grid-cols-2">
                <label className="space-y-2 text-sm font-medium">
                  {t("frequency")}
                  <select
                    className="h-11 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus:border-ring focus:ring-3 focus:ring-ring/50"
                    defaultValue="instant"
                  >
                    <option value="instant">{t("instant")}</option>
                    <option value="daily">{t("daily")}</option>
                  </select>
                </label>
                <label className="space-y-2 text-sm font-medium">
                  {t("minimumMatch")}
                  <select
                    className="h-11 w-full rounded-md border border-input bg-background px-3 text-sm outline-none focus:border-ring focus:ring-3 focus:ring-ring/50"
                    defaultValue="80"
                  >
                    <option value="70">70%</option>
                    <option value="80">80%</option>
                    <option value="90">90%</option>
                  </select>
                </label>
              </CardContent>
            </Card>
          </Reveal>

          <Reveal delay={0.2}>
            <Card>
              <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-start gap-3">
                  <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-accent text-primary">
                    <Send className="size-4" aria-hidden="true" />
                  </span>
                  <div>
                    <p className="text-sm font-semibold">
                      {t("telegramConnected")}
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      @timur_hireradar
                    </p>
                  </div>
                </div>
                <Button variant="outline">{t("disconnect")}</Button>
              </CardContent>
            </Card>
          </Reveal>

          <div className="flex justify-end">
            <SaveSettingsButton />
          </div>
        </div>
      </main>
      <MobileNavigation active="settings" />
    </div>
  );
}
