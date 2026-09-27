import { BriefcaseBusiness, Clock3, Globe2, Send } from "lucide-react";
import { getTranslations } from "next-intl/server";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { ChoiceCard } from "@/components/onboarding/choice-card";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";
import { saveOnboardingPreferences } from "@/app/profile-actions";

export default async function OnboardingPreferencesPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const t = await getTranslations("onboarding");
  const { error } = await searchParams;
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
            <CardContent className="grid gap-3 sm:grid-cols-2">
              <ChoiceCard
                name="roles"
                value="Backend Engineer"
                label={t("backend")}
                checked
              />
              <ChoiceCard
                name="roles"
                value="Frontend Engineer"
                label={t("frontend")}
                checked
              />
              <ChoiceCard
                name="roles"
                value="Full Stack Engineer"
                label={t("fullstack")}
                checked
              />
              <ChoiceCard
                name="roles"
                value="Engineering Lead"
                label={t("lead")}
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t("workPreferences")}</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3 sm:grid-cols-2">
              <ChoiceCard
                name="remotePolicy"
                value="worldwide"
                label={t("worldwide")}
                description={t("worldwideHelp")}
                icon={<Globe2 className="size-4" aria-hidden="true" />}
                checked
              />
              <ChoiceCard
                name="region"
                value="emea"
                label={t("emea")}
                description={t("emeaHelp")}
                icon={<Clock3 className="size-4" aria-hidden="true" />}
                checked
              />
              <ChoiceCard
                name="employmentType"
                value="contract"
                label={t("contract")}
                description={t("contractHelp")}
                icon={
                  <BriefcaseBusiness className="size-4" aria-hidden="true" />
                }
                checked
              />
              <ChoiceCard
                name="employmentType"
                value="full_time"
                label={t("fulltime")}
                description={t("fulltimeHelp")}
                icon={
                  <BriefcaseBusiness className="size-4" aria-hidden="true" />
                }
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t("telegram")}</CardTitle>
            </CardHeader>
            <CardContent>
              <ChoiceCard
                name="telegram"
                label={t("connectTelegram")}
                description={t("telegramHelp")}
                icon={<Send className="size-4" aria-hidden="true" />}
                checked
              />
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
