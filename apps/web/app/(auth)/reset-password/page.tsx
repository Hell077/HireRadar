import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default async function ResetPasswordPage() {
  const t = await getTranslations("auth");
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
      <form className="space-y-5">
        <AuthField
          id="password"
          label={t("newPassword")}
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder={t("enterNewPassword")}
          minLength={8}
          required
        />
        <AuthField
          id="confirm-password"
          label={t("confirmPassword")}
          name="confirmPassword"
          type="password"
          autoComplete="new-password"
          placeholder={t("repeatPassword")}
          minLength={8}
          required
        />
        <Button className="h-11 w-full" type="submit">
          {t("updatePassword")}
        </Button>
      </form>
    </AuthShell>
  );
}
