import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";
import { register } from "../actions";

export default async function RegisterPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string; sent?: string }>;
}) {
  const t = await getTranslations("auth");
  const { error, sent } = await searchParams;
  return (
    <AuthShell
      title={t("registerTitle")}
      description={t("registerDescription")}
      footer={
        <>
          {t("alreadyAccount")}{" "}
          <Link
            className="font-medium text-primary hover:underline"
            href="/sign-in"
          >
            {t("signIn")}
          </Link>
        </>
      }
    >
      <form action={register} className="space-y-5">
        {sent === "1" ? (
          <p className="rounded-lg bg-success-soft p-3 text-sm text-success">
            {t("verificationSent")}
          </p>
        ) : error ? (
          <p className="rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t(
              error === "duplicate"
                ? "emailTaken"
                : error === "validation"
                  ? "invalidRegistration"
                  : error === "limited"
                    ? "rateLimited"
                    : "serviceUnavailable",
            )}
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
        <AuthField
          id="password"
          label={t("password")}
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder={t("newPasswordPlaceholder")}
          minLength={12}
          required
        />
        <p className="text-xs leading-5 text-muted-foreground">{t("terms")}</p>
        <Button className="h-11 w-full" type="submit">
          {t("createAccount")}
        </Button>
      </form>
    </AuthShell>
  );
}
