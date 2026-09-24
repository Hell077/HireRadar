import { Card, CardContent } from "@repo/ui/components/card";
import { getTranslations } from "next-intl/server";

import { saveOnboardingProfile } from "@/app/profile-actions";
import { AuthField } from "@/components/auth/auth-field";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default async function OnboardingProfilePage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const t = await getTranslations("onboarding");
  const { error } = await searchParams;
  return (
    <OnboardingShell
      step={1}
      title={t("profileTitle")}
      description={t("profileDescription")}
    >
      <form action={saveOnboardingProfile}>
        {error ? (
          <p className="mb-5 rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t("saveError")}
          </p>
        ) : null}
        <Card>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            <AuthField
              id="first-name"
              label={t("firstName")}
              name="firstName"
              autoComplete="given-name"
              required
            />
            <AuthField
              id="last-name"
              label={t("lastName")}
              name="lastName"
              autoComplete="family-name"
              required
            />
            <AuthField
              id="seniority"
              label={t("seniority")}
              name="seniority"
              placeholder="senior"
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
              id="country"
              label={t("country")}
              name="country"
              autoComplete="country"
              placeholder="KZ"
              maxLength={2}
            />
            <AuthField
              id="city"
              label={t("city")}
              name="city"
              autoComplete="address-level2"
              placeholder={t("cityPlaceholder")}
            />
            <AuthField
              id="timezone"
              label={t("timezone")}
              name="timezone"
              placeholder="UTC+5"
            />
          </CardContent>
        </Card>
        <OnboardingActions nextHref="/onboarding/resume" submit />
      </form>
    </OnboardingShell>
  );
}
