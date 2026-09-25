import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { OnboardingActions } from "@/components/onboarding/onboarding-actions";
import { ResumeUploader } from "@/components/onboarding/resume-uploader";
import { OnboardingShell } from "@/components/onboarding/onboarding-shell";

export default async function OnboardingResumePage() {
  const t = await getTranslations("onboarding");
  return (
    <OnboardingShell
      step={2}
      title={t("resumeTitle")}
      description={t("resumeDescription")}
    >
      <ResumeUploader labels={{
        drop: t("dropResume"),
        help: t("fileHelp"),
        choose: t("chooseFile"),
        privacy: t("privacy"),
        uploading: t("uploading"),
        uploaded: t("uploaded"),
        error: t("uploadError"),
      }} />
      <p className="mt-5 text-center text-sm text-muted-foreground">
        {t("manual")} {" "}
        <Link className="font-medium text-primary hover:underline" href="/onboarding/review">
          {t("skip")}
        </Link>
      </p>
      <OnboardingActions
        backHref="/onboarding/profile"
        nextHref="/onboarding/review"
        nextLabel={t("reviewProfile")}
      />
    </OnboardingShell>
  );
}
