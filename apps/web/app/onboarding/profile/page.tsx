import { Card, CardContent } from "@repo/ui/components/card";
import { getTranslations } from "next-intl/server";

import { AuthField } from "@/components/auth/auth-field";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default async function OnboardingProfilePage() {
  const t = await getTranslations("onboarding");
  const auth = await getTranslations("auth");
  return (
    <OnboardingShell
      step={1}
      title={t("profileTitle")}
      description={t("profileDescription")}
    >
      <Card>
        <CardContent className="grid gap-5 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <AuthField
              id="full-name"
              label={auth("fullName")}
              name="fullName"
              autoComplete="name"
              defaultValue="Timur K."
            />
          </div>
          <AuthField
            id="current-role"
            label={t("currentRole")}
            name="currentRole"
            placeholder={t("rolePlaceholder")}
          />
          <AuthField
            id="experience"
            label={t("experience")}
            name="experience"
            type="number"
            min={0}
            placeholder="5"
          />
          <AuthField
            id="location"
            label={t("location")}
            name="location"
            autoComplete="country-name"
            placeholder={t("locationPlaceholder")}
          />
          <AuthField
            id="timezone"
            label={t("timezone")}
            name="timezone"
            placeholder="UTC+5"
          />
        </CardContent>
      </Card>
      <OnboardingActions nextHref="/onboarding/resume" />
    </OnboardingShell>
  );
}
