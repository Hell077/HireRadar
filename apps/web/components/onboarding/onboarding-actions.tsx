import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { ArrowLeft, ArrowRight } from "lucide-react";
import { Button } from "@repo/ui/components/button";

type OnboardingActionsProps = {
  backHref?: string;
  nextHref: string;
  nextLabel?: string;
};

export async function OnboardingActions({
  backHref,
  nextHref,
  nextLabel,
}: OnboardingActionsProps) {
  const t = await getTranslations("common");
  return (
    <div className="mt-8 flex items-center justify-between gap-4 border-t pt-6">
      {backHref ? (
        <Button asChild variant="ghost">
          <Link href={backHref}>
            <ArrowLeft aria-hidden="true" />
            {t("back")}
          </Link>
        </Button>
      ) : (
        <span />
      )}
      <Button asChild size="lg">
        <Link href={nextHref}>
          {nextLabel ?? t("continue")}
          <ArrowRight aria-hidden="true" />
        </Link>
      </Button>
    </div>
  );
}
