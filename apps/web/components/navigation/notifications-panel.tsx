"use client";

import { useEffect, useRef, useState } from "react";

import { Bell, CheckCheck, FileCheck2, Send, Sparkles, X } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useTranslations } from "next-intl";

export function NotificationsPanel() {
  const t = useTranslations("notifications");
  const [open, setOpen] = useState(false);
  const [read, setRead] = useState(false);
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

  const items = [
    { icon: Sparkles, title: t("matchTitle"), detail: t("matchDetail") },
    { icon: FileCheck2, title: t("resumeTitle"), detail: t("resumeDetail") },
    { icon: Send, title: t("telegramTitle"), detail: t("telegramDetail") },
  ];

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={t("open")}
        className="relative grid size-9.5 place-items-center rounded-full bg-secondary text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Bell className="size-4.5" aria-hidden="true" />
        {!read ? (
          <span className="absolute right-0.5 top-0.5 size-2 rounded-full bg-destructive ring-2 ring-card" />
        ) : null}
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
              role="dialog"
              aria-label={t("title")}
              initial={
                reduceMotion ? false : { opacity: 0, y: 10, scale: 0.97 }
              }
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={
                reduceMotion
                  ? { opacity: 0 }
                  : { opacity: 0, y: 8, scale: 0.98 }
              }
              className="fixed inset-x-0 bottom-0 z-50 rounded-t-2xl border bg-card p-4 pb-7 shadow-lg md:absolute md:inset-x-auto md:bottom-auto md:right-0 md:top-12 md:w-[26rem] md:rounded-xl md:p-4"
            >
              <div className="flex items-center justify-between">
                <h2 className="font-semibold">{t("title")}</h2>
                <button
                  type="button"
                  onClick={() => setRead(true)}
                  className="flex items-center gap-1.5 text-xs font-semibold text-primary"
                >
                  <CheckCheck className="size-4" aria-hidden="true" />
                  {t("markAll")}
                </button>
              </div>
              <div className="mt-3 space-y-1">
                {items.map(({ icon: Icon, title, detail }, index) => (
                  <div
                    key={title}
                    className={`flex gap-3 rounded-xl p-3 ${!read && index === 0 ? "bg-accent" : "hover:bg-secondary/70"}`}
                  >
                    <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-secondary text-primary">
                      <Icon className="size-4" aria-hidden="true" />
                    </span>
                    <div>
                      <p className="text-sm font-semibold">{title}</p>
                      <p className="mt-1 text-xs leading-5 text-muted-foreground">
                        {detail}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
              <button
                type="button"
                onClick={() => setOpen(false)}
                className="absolute right-4 top-4 rounded-md p-1 text-muted-foreground md:hidden"
              >
                <X className="size-4" aria-hidden="true" />
              </button>
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>
    </div>
  );
}
