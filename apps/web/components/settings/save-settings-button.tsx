"use client";

import { useState } from "react";

import { Check, Save } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useTranslations } from "next-intl";
import { Button } from "@repo/ui/components/button";

export function SaveSettingsButton() {
  const t = useTranslations("settings");
  const [saved, setSaved] = useState(false);
  const reduceMotion = useReducedMotion();

  function save() {
    setSaved(true);
    window.setTimeout(() => setSaved(false), 3000);
  }

  return (
    <>
      <Button size="lg" onClick={save}>
        <Save aria-hidden="true" />
        {t("save")}
      </Button>
      <AnimatePresence>
        {saved ? (
          <motion.div
            role="status"
            initial={reduceMotion ? false : { opacity: 0, y: 12, scale: 0.96 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={
              reduceMotion ? { opacity: 0 } : { opacity: 0, y: 8, scale: 0.98 }
            }
            className="fixed bottom-22 left-1/2 z-50 flex -translate-x-1/2 items-center gap-2 rounded-xl border bg-card px-4 py-3 text-sm font-semibold shadow-lg md:bottom-6"
          >
            <Check className="size-4 text-success" aria-hidden="true" />
            {t("saved")}
          </motion.div>
        ) : null}
      </AnimatePresence>
    </>
  );
}
