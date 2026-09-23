import { Check, X } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { AuthField } from "@/components/auth/auth-field";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

const extractedSkills = [
  "Go",
  "React",
  "Next.js",
  "TypeScript",
  "PostgreSQL",
  "Docker",
];

export default function OnboardingReviewPage() {
  return (
    <OnboardingShell
      step={3}
      title="Check your profile"
      description="We extracted the following information from your resume. Correct anything that does not look right."
    >
      <div className="mb-5 flex items-center gap-3 rounded-xl bg-success-soft p-4 text-success">
        <Check className="size-5 shrink-0" aria-hidden="true" />
        <p className="text-sm font-medium">Resume processed successfully</p>
      </div>

      <div className="space-y-5">
        <Card>
          <CardHeader>
            <CardTitle>Professional summary</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            <AuthField
              id="review-role"
              label="Primary role"
              name="role"
              defaultValue="Golang & Frontend Developer"
            />
            <AuthField
              id="review-experience"
              label="Years of experience"
              name="experience"
              type="number"
              defaultValue="5"
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Detected skills</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-2">
              {extractedSkills.map((skill) => (
                <span
                  key={skill}
                  className="inline-flex items-center gap-2 rounded-full bg-secondary px-3 py-2 text-sm font-medium"
                >
                  {skill}
                  <button
                    className="rounded-full text-muted-foreground hover:text-foreground"
                    type="button"
                    aria-label={`Remove ${skill}`}
                  >
                    <X className="size-3.5" aria-hidden="true" />
                  </button>
                </span>
              ))}
              <button
                className="rounded-full border border-dashed px-3 py-2 text-sm font-medium text-primary hover:bg-accent"
                type="button"
              >
                + Add skill
              </button>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Recent experience</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            <AuthField
              id="company"
              label="Company"
              name="company"
              defaultValue="Product company"
            />
            <AuthField
              id="position"
              label="Position"
              name="position"
              defaultValue="Software Engineer"
            />
          </CardContent>
        </Card>
      </div>

      <OnboardingActions
        backHref="/onboarding/resume"
        nextHref="/onboarding/preferences"
        nextLabel="Confirm details"
      />
    </OnboardingShell>
  );
}
