import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { ArrowLeft } from "lucide-react";
import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default async function ForgotPasswordPage() {
  const t = await getTranslations("auth");
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
        <Button className="h-11 w-full" type="submit">
          {t("sendReset")}
        </Button>
      </form>
    </AuthShell>
  );
}
