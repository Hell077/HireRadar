"use client";

import Link from "next/link";
import { BriefcaseBusiness, Bookmark, Settings, UserRound } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { useTranslations } from "next-intl";

type NavigationItem = "profile" | "jobs" | "saved" | "settings";

export function MobileNavigation({
  active = "profile",
}: {
  active?: NavigationItem;
}) {
  const reduceMotion = useReducedMotion();
  const t = useTranslations("nav");
  const items = [
    { key: "jobs", label: t("jobs"), href: "/jobs", icon: BriefcaseBusiness },
    { key: "saved", label: t("saved"), href: "/saved", icon: Bookmark },
    { key: "profile", label: t("profile"), href: "/", icon: UserRound },
    {
      key: "settings",
      label: t("settings"),
      href: "/settings",
      icon: Settings,
    },
  ];

  return (
    <motion.nav
      className="border-border bg-card fixed inset-x-4 bottom-3 z-20 flex h-15 items-center gap-1 rounded-full border p-1.5 shadow-[0_8px_24px_color-mix(in_srgb,var(--foreground)_10%,transparent)] md:hidden"
      aria-label="Mobile navigation"
      initial={reduceMotion ? false : { opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{
        delay: reduceMotion ? 0 : 0.18,
        duration: reduceMotion ? 0 : 0.4,
        ease: [0.22, 1, 0.36, 1],
      }}
    >
      {items.map(({ key, label, href, icon: Icon }) => (
        <Link
          key={label}
          href={href}
          className={
            key === active
              ? "bg-accent text-accent-foreground flex h-full flex-1 flex-col items-center justify-center gap-0.5 rounded-full text-[10px] font-semibold"
              : "text-muted-foreground flex h-full flex-1 flex-col items-center justify-center gap-0.5 rounded-full text-[10px] font-medium"
          }
        >
          <Icon className="size-4.5" aria-hidden="true" />
          {label}
        </Link>
      ))}
    </motion.nav>
  );
}
