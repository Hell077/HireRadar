"use client";

import { PageError } from "@/components/feedback/page-error";

export default function SavedError({
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  return <PageError reset={reset} />;
}
