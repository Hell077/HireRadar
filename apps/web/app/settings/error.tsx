"use client";

import { PageError } from "@/components/feedback/page-error";

export default function SettingsError({
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  return <PageError reset={reset} />;
}
