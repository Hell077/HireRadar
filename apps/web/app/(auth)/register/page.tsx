import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default async function RegisterPage() {
  const t = await getTranslations("auth");
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
      <form className="space-y-5">
        <AuthField
          id="name"
          label={t("fullName")}
          name="name"
          autoComplete="name"
          placeholder={t("namePlaceholder")}
          required
        />
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
          minLength={8}
          required
        />
        <p className="text-xs leading-5 text-muted-foreground">{t("terms")}</p>
        <Button asChild className="h-11 w-full">
          <Link href="/onboarding/profile">{t("createAccount")}</Link>
        </Button>
      </form>
    </AuthShell>
  );
}
