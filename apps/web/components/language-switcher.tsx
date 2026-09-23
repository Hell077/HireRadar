"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useTransition } from "react";

export function LanguageSwitcher() {
  const locale = useLocale();
  const t = useTranslations("language");
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function changeLocale(nextLocale: "en" | "ru") {
    document.cookie = `HIRERADAR_LOCALE=${nextLocale};path=/;max-age=31536000;samesite=lax`;
    startTransition(() => router.refresh());
  }

  return (
    <div className="flex rounded-lg bg-secondary p-1" aria-label={t("label")}>
      {(["en", "ru"] as const).map((item) => (
        <button
          key={item}
          type="button"
          disabled={pending}
          onClick={() => changeLocale(item)}
          className={`rounded-md px-2 py-1 text-xs font-semibold ${locale === item ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
          aria-pressed={locale === item}
        >
          {t(item)}
        </button>
      ))}
    </div>
  );
}
