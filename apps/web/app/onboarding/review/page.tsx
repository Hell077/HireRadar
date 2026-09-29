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
import { saveOnboardingProfessional } from "@/app/profile-actions";
import { getCandidateData } from "@/lib/api/server";

export default async function OnboardingReviewPage() {
  const t = await getTranslations("onboarding");
  const result = await getLatestResumeAnalysis();
  const candidate = await getCandidateData();
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
        <form id="professional-profile" action={saveOnboardingProfessional}>
          <Card className="border-0 shadow-sm">
            <CardHeader>
              <CardTitle>{t("summary")}</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-5 sm:grid-cols-2">
              <AuthField
                id="review-role"
                label={t("primaryRole")}
                name="role"
                defaultValue={
                  analysis?.positions?.[0]?.title ??
                  candidate?.positions?.[0] ??
                  ""
                }
                required
              />
              <AuthField
                id="review-experience"
                label={t("experience")}
                name="experience"
                type="number"
                defaultValue={
                  analysis
                    ? String(Math.round(analysis.total_experience_months / 12))
                    : candidate?.profile.experience_years
                }
                min={0}
                max={60}
              />
              <div className="sm:col-span-2">
                <AuthField
                  id="review-skills"
                  label={t("skillsManual")}
                  name="skills"
                  defaultValue={candidate?.skills
                    .map((skill) => skill.name)
                    .join(", ")}
                  placeholder={t("skillsPlaceholder")}
                />
              </div>
              <div className="sm:col-span-2">
                <p className="text-sm font-medium">{t("languagesDetected")}</p>
                {analysis?.languages?.length ? (
                  <div className="mt-2 flex flex-wrap gap-2">
                    {analysis.languages.map((language) => (
                      <span
                        className="rounded-full bg-secondary px-3 py-1 text-sm font-medium"
                        key={language}
                      >
                        {language.toUpperCase()}
                      </span>
                    ))}
                  </div>
                ) : (
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t("noLanguagesDetected")}
                  </p>
                )}
                <p className="mt-2 text-sm text-muted-foreground">
                  {t("languageFilterDescription")}
                </p>
              </div>
            </CardContent>
          </Card>
        </form>

        <Card>
          <CardHeader>
            <CardTitle>{t("skills")}</CardTitle>
          </CardHeader>
          <CardContent>
            {result?.resume && analysis ? (
              <ResumeSuggestions
                resumeID={result.resume.id}
                suggestions={result.suggestions}
                labels={{
                  accept: t("acceptSuggestion"),
                  reject: t("rejectSuggestion"),
                  accepted: t("suggestionAccepted"),
                  rejected: t("suggestionRejected"),
                  failed: t("suggestionReviewError"),
                }}
              />
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("noSuggestions")}
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("recentExperience")}</CardTitle>
          </CardHeader>
          <CardContent className="grid gap-5 sm:grid-cols-2">
            {analysis?.positions?.length ? (
              analysis.positions.map((position) => (
                <p
                  className="rounded-lg bg-secondary/50 p-3 text-sm"
                  key={position.title}
                >
                  {position.title}
                </p>
              ))
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("noSuggestions")}
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      <OnboardingActions
        backHref="/onboarding/resume"
        form="professional-profile"
        nextHref="/onboarding/preferences"
        nextLabel={t("confirm")}
        submit
      />
    </OnboardingShell>
  );
}
