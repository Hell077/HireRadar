import { Card, CardContent } from "@repo/ui/components/card";

import { AuthField } from "@/components/auth/auth-field";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default function OnboardingProfilePage() {
  return (
    <OnboardingShell
      step={1}
      title="Tell us what you do"
      description="This information gives the resume parser a useful starting point and improves your first recommendations."
    >
      <Card>
        <CardContent className="grid gap-5 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <AuthField
              id="full-name"
              label="Full name"
              name="fullName"
              autoComplete="name"
              defaultValue="Timur K."
            />
          </div>
          <AuthField
            id="current-role"
            label="Current role"
            name="currentRole"
            placeholder="Golang developer"
          />
          <AuthField
            id="experience"
            label="Years of experience"
            name="experience"
            type="number"
            min={0}
            placeholder="5"
          />
          <AuthField
            id="location"
            label="Current location"
            name="location"
            autoComplete="country-name"
            placeholder="Kazakhstan"
          />
          <AuthField
            id="timezone"
            label="Time zone"
            name="timezone"
            placeholder="UTC+5"
          />
        </CardContent>
      </Card>
      <OnboardingActions nextHref="/onboarding/resume" />
    </OnboardingShell>
  );
}
