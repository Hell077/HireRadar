"use client";

import type { ReactNode } from "react";

import { motion, useReducedMotion } from "motion/react";

export function RouteTransition({ children }: { children: ReactNode }) {
  const reduceMotion = useReducedMotion();

  return (
    <motion.div
      initial={reduceMotion ? false : { opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: reduceMotion ? 0 : 0.24, ease: "easeOut" }}
    >
      {children}
    </motion.div>
  );
}
