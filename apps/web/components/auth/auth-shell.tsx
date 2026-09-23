import Link from "next/link";
import type { ReactNode } from "react";
import { getTranslations } from "next-intl/server";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@repo/ui/components/card";

import { Reveal } from "@/components/motion/reveal";
import { LanguageSwitcher } from "@/components/language-switcher";

type AuthShellProps = {
  children: ReactNode;
  description: string;
  footer: ReactNode;
  title: string;
};

export async function AuthShell({
  children,
  description,
  footer,
  title,
}: AuthShellProps) {
  const t = await getTranslations("auth");

  return (
    <main className="grid min-h-svh bg-background lg:grid-cols-[minmax(0,1fr)_minmax(32rem,0.8fr)]">
      <section className="relative hidden overflow-hidden bg-primary p-12 text-primary-foreground lg:flex lg:flex-col lg:justify-between">
        <Reveal>
          <Link
            href="/"
            className="flex w-fit items-center gap-3 text-lg font-semibold"
          >
            <span className="grid size-10 place-items-center rounded-xl bg-primary-foreground/10 ring-1 ring-primary-foreground/20">
              HR
            </span>
            HireRadar
          </Link>
        </Reveal>

        <Reveal className="max-w-xl space-y-5" delay={0.08} distance={20}>
          <p className="text-sm font-medium uppercase tracking-[0.18em] text-primary-foreground/70">
            {t("eyebrow")}
          </p>
          <h2 className="text-balance text-4xl font-semibold leading-tight xl:text-5xl">
            {t("heroTitle")}
          </h2>
          <p className="max-w-lg text-lg leading-8 text-primary-foreground/75">
            {t("heroText")}
          </p>
        </Reveal>

        <p className="text-sm text-primary-foreground/60">{t("heroFooter")}</p>
      </section>

      <section className="flex min-h-svh flex-col">
        <header className="flex h-20 items-center justify-between px-5 sm:px-8 lg:hidden">
          <Link href="/" className="flex items-center gap-2.5 font-semibold">
            <span className="grid size-9 place-items-center rounded-lg bg-primary text-xs text-primary-foreground">
              HR
            </span>
            HireRadar
          </Link>
          <LanguageSwitcher />
        </header>

        <div className="flex flex-1 items-center justify-center px-5 py-10 sm:px-8">
          <Reveal className="w-full max-w-md" delay={0.06} distance={18}>
            <Card className="border-border/80 shadow-lg shadow-foreground/5">
              <CardHeader className="space-y-2 px-6 sm:px-8">
                <CardTitle className="text-2xl tracking-tight sm:text-3xl">
                  {title}
                </CardTitle>
                <CardDescription className="text-base leading-6">
                  {description}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6 px-6 sm:px-8">
                {children}
                <div className="border-t pt-6 text-center text-sm text-muted-foreground">
                  {footer}
                </div>
              </CardContent>
            </Card>
          </Reveal>
        </div>
      </section>
    </main>
  );
}
