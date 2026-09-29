import { BriefcaseBusiness, Clock3, Globe2, Send } from "lucide-react";
import { getLocale, getTranslations } from "next-intl/server";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { ChoiceCard } from "@/components/onboarding/choice-card";
import { PositionSelector } from "@/components/preferences/position-selector";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";
import { saveOnboardingPreferences } from "@/app/profile-actions";
import { getCandidateData } from "@/lib/api/server";

export default async function OnboardingPreferencesPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const t = await getTranslations("onboarding");
  const locale = await getLocale();
  const { error } = await searchParams;
  const candidate = await getCandidateData();
  const countryCode = candidate?.profile.country || "KZ";
  const countryName =
    new Intl.DisplayNames([locale], { type: "region" }).of(countryCode) ||
    countryCode;
  return (
    <OnboardingShell
      step={4}
      title={t("preferencesTitle")}
      description={t("preferencesDescription")}
    >
      <form action={saveOnboardingPreferences}>
        {error ? (
          <p className="mb-5 rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t("preferencesError")}
          </p>
        ) : null}
        <div className="space-y-5">
          <Card>
            <CardHeader>
              <CardTitle>{t("targetRoles")}</CardTitle>
            </CardHeader>
            <CardContent>
              <PositionSelector initialPositions={candidate?.positions ?? []} />
            </CardContent>
          </Card>

          {["worldwide", "remote", "remote_region", "remote_country"].map(
            (policy) => (
              <input
                key={policy}
                type="hidden"
                name="remotePolicy"
                value={policy}
              />
            ),
          )}
          <div className="flex items-start gap-3 rounded-xl bg-secondary/50 p-4">
            <Globe2
              className="mt-0.5 size-5 shrink-0 text-primary"
              aria-hidden="true"
            />
            <div>
              <p className="text-sm font-semibold">{t("eligibilityTitle")}</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {t("eligibilityHelp", {
                  country: countryName,
                })}
              </p>
            </div>
          </div>

          <Card>
            <CardHeader>
              <CardTitle>{t("employmentTypes")}</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3 sm:grid-cols-2">
              <ChoiceCard
                name="employmentType"
                value="full_time"
                label={t("workFullTime")}
                description={t("fulltimeHelp")}
                icon={
                  <BriefcaseBusiness className="size-4" aria-hidden="true" />
                }
              />
              <ChoiceCard
                name="employmentType"
                value="part_time"
                label={t("workPartTime")}
                description={t("parttimeHelp")}
                icon={<Clock3 className="size-4" aria-hidden="true" />}
                checked
              />
              <ChoiceCard
                name="employmentType"
                value="project"
                label={t("workProject")}
                description={t("workProjectHelp")}
                icon={
                  <BriefcaseBusiness className="size-4" aria-hidden="true" />
                }
                checked
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t("telegram")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-start gap-3 rounded-xl bg-secondary/50 p-4">
                <Send
                  className="mt-0.5 size-5 shrink-0 text-primary"
                  aria-hidden="true"
                />
                <div>
                  <p className="text-sm font-semibold">
                    {t("telegramNotConnected")}
                  </p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t("telegramHelp")}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        <OnboardingActions
          backHref="/onboarding/review"
          nextHref="/"
          nextLabel={t("finish")}
          submit
        />
      </form>
    </OnboardingShell>
  );
}
