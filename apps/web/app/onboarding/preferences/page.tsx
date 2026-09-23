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

export default async function OnboardingPreferencesPage() {
  const t = await getTranslations("onboarding");
  return (
    <OnboardingShell
      step={4}
      title={t("preferencesTitle")}
      description={t("preferencesDescription")}
    >
      <div className="space-y-5">
        <Card>
          <CardHeader>
            <CardTitle>{t("targetRoles")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <ChoiceCard name="roles" label={t("backend")} checked />
            <ChoiceCard name="roles" label={t("frontend")} checked />
            <ChoiceCard name="roles" label={t("fullstack")} checked />
            <ChoiceCard name="roles" label={t("lead")} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("workPreferences")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <ChoiceCard
              name="region"
              label={t("worldwide")}
              description={t("worldwideHelp")}
              icon={<Globe2 className="size-4" aria-hidden="true" />}
              checked
            />
            <ChoiceCard
              name="region"
              label={t("emea")}
              description={t("emeaHelp")}
              icon={<Clock3 className="size-4" aria-hidden="true" />}
              checked
            />
            <ChoiceCard
              name="employment"
              label={t("contract")}
              description={t("contractHelp")}
              icon={<BriefcaseBusiness className="size-4" aria-hidden="true" />}
              checked
            />
            <ChoiceCard
              name="employment"
              label={t("fulltime")}
              description={t("fulltimeHelp")}
              icon={<BriefcaseBusiness className="size-4" aria-hidden="true" />}
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
      />
    </OnboardingShell>
  );
}
