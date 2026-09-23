import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { FileText, ShieldCheck, Upload } from "lucide-react";
import { Button } from "@repo/ui/components/button";
import { Card, CardContent } from "@repo/ui/components/card";

import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default async function OnboardingResumePage() {
  const t = await getTranslations("onboarding");
  return (
    <OnboardingShell
      step={2}
      title={t("resumeTitle")}
      description={t("resumeDescription")}
    >
      <Card>
        <CardContent>
          <label className="flex min-h-64 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed bg-secondary/40 px-6 py-10 text-center transition-colors hover:bg-accent/50">
            <input className="sr-only" type="file" accept=".pdf,.doc,.docx" />
            <span className="grid size-12 place-items-center rounded-xl bg-card text-primary shadow-sm">
              <Upload className="size-5" aria-hidden="true" />
            </span>
            <span className="mt-5 text-base font-semibold">
              {t("dropResume")}
            </span>
            <span className="mt-2 text-sm text-muted-foreground">
              {t("fileHelp")}
            </span>
            <Button className="mt-5" type="button" variant="outline">
              <FileText aria-hidden="true" />
              {t("chooseFile")}
            </Button>
          </label>

          <div className="mt-5 flex items-start gap-3 rounded-lg bg-success-soft p-4 text-success">
            <ShieldCheck
              className="mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
            <p className="text-xs leading-5">{t("privacy")}</p>
          </div>

          <p className="mt-5 text-center text-sm text-muted-foreground">
            {t("manual")}{" "}
            <Link
              className="font-medium text-primary hover:underline"
              href="/onboarding/review"
            >
              {t("skip")}
            </Link>
          </p>
        </CardContent>
      </Card>
      <OnboardingActions
        backHref="/onboarding/profile"
        nextHref="/onboarding/review"
        nextLabel={t("reviewProfile")}
      />
    </OnboardingShell>
  );
}
