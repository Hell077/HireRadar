"use client";

import Link from "next/link";
import { Radar } from "lucide-react";
import { useTranslations } from "next-intl";

import { LanguageSwitcher } from "@/components/language-switcher";
import { AccountMenu } from "@/components/navigation/account-menu";
import { NotificationsPanel } from "@/components/navigation/notifications-panel";

type NavigationItem = "profile" | "jobs" | "saved" | "settings";

export function AppHeader({ active = "profile" }: { active?: NavigationItem }) {
  const t = useTranslations("nav");
  const navigation = [
    { key: "profile", label: t("profile"), href: "/" },
    { key: "jobs", label: t("jobs"), href: "/jobs" },
    { key: "saved", label: t("saved"), href: "/saved" },
    { key: "settings", label: t("settings"), href: "/settings" },
  ];
  return (
    <header className="border-border bg-card relative z-30 border-b">
      <div className="mx-auto flex h-18 max-w-[1440px] items-center justify-between px-5 md:px-14">
        <Link
          href="/"
          className="flex items-center gap-2.5"
          aria-label="HireRadar home"
        >
          <span className="bg-primary text-primary-foreground grid size-8 place-items-center rounded-[10px]">
            <Radar className="size-4.5" aria-hidden="true" />
          </span>
          <span className="text-[17px] font-bold tracking-tight">
            HireRadar
          </span>
        </Link>

        <nav
          className="hidden items-center gap-2 md:flex"
          aria-label="Primary navigation"
        >
          {navigation.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={
                item.key === active
                  ? "bg-accent text-accent-foreground rounded-lg px-3.5 py-2 text-sm font-semibold"
                  : "text-muted-foreground hover:bg-secondary hover:text-foreground rounded-lg px-3.5 py-2 text-sm font-medium transition-colors"
              }
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="flex items-center gap-2.5">
          <LanguageSwitcher />
          <NotificationsPanel />
          <AccountMenu />
        </div>
      </div>
    </header>
  );
}
