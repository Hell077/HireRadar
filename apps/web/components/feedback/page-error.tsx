"use client";

import { CircleAlert, RotateCcw } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@repo/ui/components/button";

export function PageError({ reset }: { reset: () => void }) {
  const t = useTranslations("states");
  return (
    <div className="flex min-h-[60svh] items-center justify-center px-4">
      <div className="flex max-w-md flex-col items-center text-center">
        <span className="grid size-12 place-items-center rounded-xl bg-warning-soft text-warning">
          <CircleAlert className="size-5" aria-hidden="true" />
        </span>
        <h1 className="mt-5 text-xl font-semibold">{t("errorTitle")}</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          {t("errorDescription")}
        </p>
        <Button className="mt-5" onClick={reset}>
          <RotateCcw aria-hidden="true" />
          {t("retry")}
        </Button>
      </div>
    </div>
  );
}
