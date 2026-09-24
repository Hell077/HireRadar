import type { ReactNode } from "react";

import Link from "next/link";

import { Button } from "@repo/ui/components/button";

type StatusStateProps = {
  actionHref?: string;
  actionLabel?: string;
  description: string;
  icon: ReactNode;
  title: string;
};

export function StatusState({
  actionHref,
  actionLabel,
  description,
  icon,
  title,
}: StatusStateProps) {
  return (
    <div className="flex min-h-80 flex-col items-center justify-center rounded-xl border border-dashed bg-card px-6 py-12 text-center">
      <span className="grid size-12 place-items-center rounded-xl bg-accent text-primary">
        {icon}
      </span>
      <h2 className="mt-5 text-lg font-semibold">{title}</h2>
      <p className="mt-2 max-w-md text-sm leading-6 text-muted-foreground">
        {description}
      </p>
      {actionHref && actionLabel ? (
        <Button asChild className="mt-5">
          <Link href={actionHref}>{actionLabel}</Link>
        </Button>
      ) : null}
    </div>
  );
}
