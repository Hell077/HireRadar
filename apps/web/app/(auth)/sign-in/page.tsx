import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default async function SignInPage() {
  const t = await getTranslations("auth");
  return (
    <AuthShell
      title={t("signInTitle")}
      description={t("signInDescription")}
      footer={
        <>
          {t("newHere")}{" "}
          <Link
            className="font-medium text-primary hover:underline"
            href="/register"
          >
            {t("createAccount")}
          </Link>
        </>
      }
    >
      <form className="space-y-5">
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
          autoComplete="current-password"
          placeholder={t("passwordPlaceholder")}
          required
          trailing={
            <Link
              className="text-sm text-primary hover:underline"
              href="/forgot-password"
            >
              {t("forgotPassword")}
            </Link>
          }
        />
        <Button className="h-11 w-full" type="submit">
          {t("signIn")}
        </Button>
      </form>
    </AuthShell>
  );
}
