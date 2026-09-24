"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import {
  LogOut,
  Monitor,
  Moon,
  Settings,
  Sun,
  UserRound,
  X,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useTranslations } from "next-intl";

import { useTheme, type Theme } from "@/components/theme-provider";
import { signOut } from "@/app/profile-actions";

const themeIcons = { light: Sun, dark: Moon, system: Monitor };

export function AccountMenu() {
  const t = useTranslations("accountMenu");
  const { theme, setTheme } = useTheme();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const reduceMotion = useReducedMotion();

  useEffect(() => {
    function close(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    function escape(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", escape);
    };
  }, []);

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label={t("open")}
        className="grid size-9.5 place-items-center rounded-full bg-primary text-xs font-bold text-primary-foreground ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      >
        T
      </button>
      <AnimatePresence>
        {open ? (
          <>
            <motion.button
              aria-label={t("close")}
              className="fixed inset-0 z-40 bg-foreground/35 backdrop-blur-[2px] md:hidden"
              onClick={() => setOpen(false)}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
            />
            <motion.div
              role="menu"
              initial={
                reduceMotion ? false : { opacity: 0, y: 10, scale: 0.97 }
              }
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={
                reduceMotion
                  ? { opacity: 0 }
                  : { opacity: 0, y: 8, scale: 0.98 }
              }
              transition={{ duration: reduceMotion ? 0 : 0.18 }}
              className="fixed inset-x-0 bottom-0 z-50 rounded-t-2xl border bg-card p-4 pb-7 shadow-lg md:absolute md:inset-x-auto md:bottom-auto md:right-0 md:top-12 md:w-80 md:rounded-xl md:p-3"
            >
              <div className="mb-3 flex items-start justify-between px-2 pt-1">
                <div>
                  <p className="text-sm font-semibold">Timur K.</p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    timur@example.com
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  className="rounded-md p-1 text-muted-foreground hover:bg-secondary md:hidden"
                >
                  <X className="size-4" aria-hidden="true" />
                </button>
              </div>
              <div className="border-t py-2">
                <Link
                  role="menuitem"
                  href="/"
                  onClick={() => setOpen(false)}
                  className="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium hover:bg-secondary"
                >
                  <UserRound
                    className="size-4 text-muted-foreground"
                    aria-hidden="true"
                  />
                  {t("profile")}
                </Link>
                <Link
                  role="menuitem"
                  href="/settings"
                  onClick={() => setOpen(false)}
                  className="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium hover:bg-secondary"
                >
                  <Settings
                    className="size-4 text-muted-foreground"
                    aria-hidden="true"
                  />
                  {t("settings")}
                </Link>
              </div>
              <div className="border-t py-3">
                <p className="px-3 text-xs font-semibold text-muted-foreground">
                  {t("theme")}
                </p>
                <div className="mt-2 grid grid-cols-3 gap-1 rounded-lg bg-secondary p-1">
                  {(["light", "dark", "system"] as Theme[]).map((item) => {
                    const Icon = themeIcons[item];
                    return (
                      <button
                        key={item}
                        type="button"
                        onClick={() => setTheme(item)}
                        aria-pressed={theme === item}
                        className={`flex items-center justify-center gap-1.5 rounded-md px-2 py-2 text-xs font-semibold ${theme === item ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground"}`}
                      >
                        <Icon className="size-3.5" aria-hidden="true" />
                        {t(item)}
                      </button>
                    );
                  })}
                </div>
              </div>
              <form action={signOut} className="border-t">
                <button
                  role="menuitem"
                  type="submit"
                  className="flex w-full items-center gap-3 px-3 py-3 text-sm font-semibold text-destructive"
                >
                  <LogOut className="size-4" aria-hidden="true" />
                  {t("signOut")}
                </button>
              </form>
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>
    </div>
  );
}
