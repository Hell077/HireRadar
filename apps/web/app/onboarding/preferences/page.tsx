import { BriefcaseBusiness, Clock3, Globe2, Send } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { ChoiceCard } from "@/components/onboarding/choice-card";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default function OnboardingPreferencesPage() {
  return (
    <OnboardingShell
      step={4}
      title="Set your search preferences"
      description="Choose what should count as a strong match. You can change these settings at any time."
    >
      <div className="space-y-5">
        <Card>
          <CardHeader>
            <CardTitle>Target roles</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <ChoiceCard name="roles" label="Golang / Backend" checked />
            <ChoiceCard name="roles" label="Frontend / React" checked />
            <ChoiceCard name="roles" label="Full stack" checked />
            <ChoiceCard name="roles" label="Engineering lead" />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Work preferences</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <ChoiceCard
              name="region"
              label="Worldwide remote"
              description="No country restriction"
              icon={<Globe2 className="size-4" aria-hidden="true" />}
              checked
            />
            <ChoiceCard
              name="region"
              label="EMEA"
              description="Europe, Middle East and Africa"
              icon={<Clock3 className="size-4" aria-hidden="true" />}
              checked
            />
            <ChoiceCard
              name="employment"
              label="Contract / B2B"
              description="Suitable for a second engagement"
              icon={<BriefcaseBusiness className="size-4" aria-hidden="true" />}
              checked
            />
            <ChoiceCard
              name="employment"
              label="Full-time"
              description="Remote permanent positions"
              icon={<BriefcaseBusiness className="size-4" aria-hidden="true" />}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Telegram notifications</CardTitle>
          </CardHeader>
          <CardContent>
            <ChoiceCard
              name="telegram"
              label="Connect Telegram after setup"
              description="Receive high-quality matches without checking the website. We’ll open the bot after you finish."
              icon={<Send className="size-4" aria-hidden="true" />}
              checked
            />
          </CardContent>
        </Card>
      </div>

      <OnboardingActions
        backHref="/onboarding/review"
        nextHref="/"
        nextLabel="Finish setup"
      />
    </OnboardingShell>
  );
}
