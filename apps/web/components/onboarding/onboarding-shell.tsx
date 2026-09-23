import Link from "next/link";
import type { ReactNode } from "react";

import { Check } from "lucide-react";

const steps = ["Profile", "Resume", "Review", "Preferences"];

type OnboardingShellProps = {
  children: ReactNode;
  description: string;
  step: number;
  title: string;
};

export function OnboardingShell({
  children,
  description,
  step,
  title,
}: OnboardingShellProps) {
  return (
    <main className="min-h-svh bg-background">
      <header className="border-b bg-card">
        <div className="mx-auto flex h-18 w-full max-w-5xl items-center justify-between px-5 sm:px-8">
          <Link href="/" className="flex items-center gap-2.5 font-semibold">
            <span className="grid size-9 place-items-center rounded-lg bg-primary text-xs text-primary-foreground">
              HR
            </span>
            HireRadar
          </Link>
          <span className="text-sm text-muted-foreground">
            Step {step} of 4
          </span>
        </div>
      </header>

      <div className="mx-auto w-full max-w-3xl px-5 py-8 sm:px-8 sm:py-12">
        <ol
          className="mb-10 grid grid-cols-4"
          aria-label="Profile setup progress"
        >
          {steps.map((label, index) => {
            const number = index + 1;
            const complete = number < step;
            const active = number === step;

            return (
              <li
                key={label}
                className="relative flex flex-col items-center gap-2"
              >
                {index > 0 ? (
                  <span
                    className={`absolute right-1/2 top-4 h-px w-full ${number <= step ? "bg-primary" : "bg-border"}`}
                    aria-hidden="true"
                  />
                ) : null}
                <span
                  className={`relative z-10 grid size-8 place-items-center rounded-full border text-xs font-semibold ${
                    complete || active
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border bg-card text-muted-foreground"
                  }`}
                  aria-current={active ? "step" : undefined}
                >
                  {complete ? (
                    <Check className="size-4" aria-hidden="true" />
                  ) : (
                    number
                  )}
                </span>
                <span
                  className={`hidden text-xs font-medium sm:block ${active ? "text-foreground" : "text-muted-foreground"}`}
                >
                  {label}
                </span>
              </li>
            );
          })}
        </ol>

        <div className="mb-8">
          <h1 className="text-balance text-3xl font-semibold tracking-tight sm:text-4xl">
            {title}
          </h1>
          <p className="mt-3 max-w-2xl text-base leading-7 text-muted-foreground">
            {description}
          </p>
        </div>

        {children}
      </div>
    </main>
  );
}
