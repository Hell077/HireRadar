import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";
import { resetPassword } from "../actions";

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string; error?: string; done?: string }>;
}) {
  const t = await getTranslations("auth");
  const { token = "", error, done } = await searchParams;
  return (
    <AuthShell
      title={t("resetTitle")}
      description={t("resetDescription")}
      footer={
        <Link
          className="font-medium text-primary hover:underline"
          href="/sign-in"
        >
          {t("returnSignIn")}
        </Link>
      }
    >
      <form action={resetPassword} className="space-y-5">
        <input type="hidden" name="token" value={token} />
        {done === "1" ? (
          <p className="rounded-lg bg-success-soft p-3 text-sm text-success">
            {t("passwordUpdated")}
          </p>
        ) : error ? (
          <p className="rounded-lg bg-destructive-soft p-3 text-sm text-destructive">
            {t(
              error === "invalid" || error === "validation"
                ? "invalidReset"
                : "serviceUnavailable",
            )}
          </p>
        ) : null}
        <AuthField
          id="password"
          label={t("newPassword")}
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder={t("enterNewPassword")}
          minLength={12}
          required
        />
        <AuthField
          id="confirm-password"
          label={t("confirmPassword")}
          name="confirmPassword"
          type="password"
          autoComplete="new-password"
          placeholder={t("repeatPassword")}
          minLength={12}
          required
        />
        <Button className="h-11 w-full" type="submit">
          {t("updatePassword")}
        </Button>
      </form>
    </AuthShell>
  );
}
