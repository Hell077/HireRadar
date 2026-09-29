import { Card, CardContent } from "@repo/ui/components/card";
import { getLocale, getTranslations } from "next-intl/server";

import { saveOnboardingProfile } from "@/app/profile-actions";
import { AuthField } from "@/components/auth/auth-field";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";
import { getCandidateData } from "@/lib/api/server";

export default async function OnboardingProfilePage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const t = await getTranslations("onboarding");
  const locale = await getLocale();
  const { error } = await searchParams;
  const data = await getCandidateData();
  const profile = data?.profile;
  const regionNames = new Intl.DisplayNames([locale], { type: "region" });
  const countries = ["KZ", "KG", "UZ", "AZ", "AM", "GE", "UA", "RU"].map(
    (code) => ({ code, label: regionNames.of(code) ?? code }),
  );
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
        <Card className="border-0 shadow-sm">
          <CardContent className="grid gap-5 sm:grid-cols-2">
            <AuthField
              id="first-name"
              label={t("firstName")}
              name="firstName"
              autoComplete="given-name"
              defaultValue={profile?.first_name}
              required
            />
            <AuthField
              id="last-name"
              label={t("lastName")}
              name="lastName"
              autoComplete="family-name"
              defaultValue={profile?.last_name}
              required
            />
            <label
              className="grid gap-2 text-sm font-medium"
              htmlFor="seniority"
            >
              {t("seniority")}
              <select
                className="h-11 rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
                id="seniority"
                name="seniority"
                defaultValue={profile?.seniority ?? ""}
              >
                <option value="">{t("chooseSeniority")}</option>
                <option value="junior">Junior</option>
                <option value="middle">Middle</option>
                <option value="senior">Senior</option>
                <option value="lead">Lead</option>
              </select>
            </label>
            <AuthField
              id="experience"
              label={t("experience")}
              name="experience"
              type="number"
              min={0}
              max={60}
              defaultValue={profile?.experience_years}
            />
            <label className="grid gap-2 text-sm font-medium" htmlFor="country">
              {t("country")}
              <select
                className="h-11 rounded-lg border border-input bg-background px-3 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
                id="country"
                name="country"
                defaultValue={profile?.country || "KZ"}
              >
                {countries.map(({ code, label }) => (
                  <option key={code} value={code}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <AuthField
              id="city"
              label={t("city")}
              name="city"
              autoComplete="address-level2"
              placeholder={t("cityPlaceholder")}
              defaultValue={profile?.city}
            />
            <input
              type="hidden"
              name="timezone"
              value={profile?.timezone || "Asia/Almaty"}
            />
          </CardContent>
        </Card>
        <OnboardingActions nextHref="/onboarding/resume" submit />
      </form>
    </OnboardingShell>
  );
}
