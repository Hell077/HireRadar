import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { ArrowLeft } from "lucide-react";
import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";
import { requestPasswordReset } from "../actions";

export default async function ForgotPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string; sent?: string }>;
}) {
  const t = await getTranslations("auth");
  const { error, sent } = await searchParams;
  return (
    <AuthShell
      title={t("forgotTitle")}
      description={t("forgotDescription")}
      footer={
        <Link
          className="inline-flex items-center gap-2 font-medium text-primary hover:underline"
          href="/sign-in"
        >
          <ArrowLeft className="size-4" />
          {t("backSignIn")}
        </Link>
      }
    >
      <form action={requestPasswordReset} className="space-y-5">
        {sent === "1" ? (
          <p className="rounded-lg bg-success-soft p-3 text-sm text-success">
            {t("resetSent")}
          </p>
        ) : error ? (
          <p className="rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t("serviceUnavailable")}
          </p>
        ) : null}
        <AuthField
          id="email"
          label={t("email")}
          name="email"
          type="email"
          autoComplete="email"
          placeholder={t("emailPlaceholder")}
          required
        />
        <Button className="h-11 w-full" type="submit">
          {t("sendReset")}
        </Button>
      </form>
    </AuthShell>
  );
}
