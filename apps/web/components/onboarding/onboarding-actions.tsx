import Link from "next/link";

import { ArrowLeft, ArrowRight } from "lucide-react";
import { Button } from "@repo/ui/components/button";

type OnboardingActionsProps = {
  backHref?: string;
  nextHref: string;
  nextLabel?: string;
};

export function OnboardingActions({
  backHref,
  nextHref,
  nextLabel = "Continue",
}: OnboardingActionsProps) {
  return (
    <div className="mt-8 flex items-center justify-between gap-4 border-t pt-6">
      {backHref ? (
        <Button asChild variant="ghost">
          <Link href={backHref}>
            <ArrowLeft aria-hidden="true" />
            Back
          </Link>
        </Button>
      ) : (
        <span />
      )}
      <Button asChild size="lg">
        <Link href={nextHref}>
          {nextLabel}
          <ArrowRight aria-hidden="true" />
        </Link>
      </Button>
    </div>
  );
}
