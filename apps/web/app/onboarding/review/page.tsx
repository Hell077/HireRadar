import { Check, X } from "lucide-react";
import { getTranslations } from "next-intl/server";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { AuthField } from "@/components/auth/auth-field";
import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";
import { ResumeSuggestions } from "@/components/onboarding/resume-suggestions";
import { getLatestResumeAnalysis } from "@/app/resume-actions";

export default async function OnboardingReviewPage() {
  const t = await getTranslations("onboarding");
  const result = await getLatestResumeAnalysis();
  const analysis = result?.analysis;
  return (
    <OnboardingShell
      step={3}
      title={t("reviewTitle")}
      description={t("reviewDescription")}
    >
      {analysis ? (
        <div className="mb-5 flex items-center gap-3 rounded-xl bg-success-soft p-4 text-success">
          <Check className="size-5 shrink-0" aria-hidden="true" />
          <p className="text-sm font-medium">{t("processed")}</p>
        </div>
      ) : (
        <p className="mb-5 rounded-xl bg-secondary/50 p-4 text-sm text-muted-foreground">
          {result ? t("processing") : t("noResume")}
        </p>
      )}

      <div className="space-y-5">
        <Card>
          <CardHeader>
            <CardTitle>{t("summary")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            <AuthField
              id="review-role"
              label={t("primaryRole")}
              name="role"
              defaultValue={analysis?.positions?.[0]?.title ?? ""}
              readOnly
            />
            <AuthField
              id="review-experience"
              label={t("experience")}
              name="experience"
              type="number"
              defaultValue={analysis ? String(Math.round(analysis.total_experience_months / 12)) : ""}
              readOnly
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("skills")}</CardTitle>
          </CardHeader>
          <CardContent>
            {result?.resume && analysis ? (
              <ResumeSuggestions resumeID={result.resume.id} suggestions={result.suggestions} labels={{
                accept: t("acceptSuggestion"), reject: t("rejectSuggestion"), accepted: t("suggestionAccepted"),
                rejected: t("suggestionRejected"), failed: t("suggestionReviewError"),
              }} />
            ) : <p className="text-sm text-muted-foreground">{t("noSuggestions")}</p>}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("recentExperience")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            {analysis?.positions?.length ? analysis.positions.map((position) => (
              <p className="rounded-lg bg-secondary/50 p-3 text-sm" key={position.title}>{position.title}</p>
            )) : <p className="text-sm text-muted-foreground">{t("noSuggestions")}</p>}
          </CardContent>
        </Card>
      </div>

      <OnboardingActions
        backHref="/onboarding/resume"
        nextHref="/onboarding/preferences"
        nextLabel={t("confirm")}
      />
    </OnboardingShell>
  );
}
