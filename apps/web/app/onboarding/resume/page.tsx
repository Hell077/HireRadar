import Link from "next/link";

import { FileText, ShieldCheck, Upload } from "lucide-react";
import { Button } from "@repo/ui/components/button";
import { Card, CardContent } from "@repo/ui/components/card";

import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default function OnboardingResumePage() {
  return (
    <OnboardingShell
      step={2}
      title="Add your resume"
      description="We’ll extract your roles, experience and skills. You can review every field before it becomes part of your profile."
    >
      <Card>
        <CardContent>
          <label className="flex min-h-64 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed bg-secondary/40 px-6 py-10 text-center transition-colors hover:bg-accent/50">
            <input className="sr-only" type="file" accept=".pdf,.doc,.docx" />
            <span className="grid size-12 place-items-center rounded-xl bg-card text-primary shadow-sm">
              <Upload className="size-5" aria-hidden="true" />
            </span>
            <span className="mt-5 text-base font-semibold">
              Drop your resume here
            </span>
            <span className="mt-2 text-sm text-muted-foreground">
              PDF or DOCX, up to 10 MB
            </span>
            <Button className="mt-5" type="button" variant="outline">
              <FileText aria-hidden="true" />
              Choose file
            </Button>
          </label>

          <div className="mt-5 flex items-start gap-3 rounded-lg bg-success-soft p-4 text-success">
            <ShieldCheck
              className="mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
            <p className="text-xs leading-5">
              Your resume is private and is used only to build your profile and
              rank vacancies. You can replace or delete it later.
            </p>
          </div>

          <p className="mt-5 text-center text-sm text-muted-foreground">
            Prefer manual setup?{" "}
            <Link
              className="font-medium text-primary hover:underline"
              href="/onboarding/review"
            >
              Skip upload
            </Link>
          </p>
        </CardContent>
      </Card>
      <OnboardingActions
        backHref="/onboarding/profile"
        nextHref="/onboarding/review"
        nextLabel="Review profile"
      />
    </OnboardingShell>
  );
}
