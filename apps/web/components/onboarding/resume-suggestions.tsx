"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@repo/ui/components/button";
import type { ResumeSuggestion } from "@/app/resume-actions";
import { reviewResumeSuggestion } from "@/app/resume-actions";

type Props = {
  resumeID: string;
  suggestions: ResumeSuggestion[];
  labels: { accept: string; reject: string; accepted: string; rejected: string; failed: string };
};

export function ResumeSuggestions({ resumeID, suggestions, labels }: Props) {
  const router = useRouter();
  const [busy, startTransition] = useTransition();
  const [error, setError] = useState(false);

  function review(suggestionID: string, accept: boolean) {
    setError(false);
    startTransition(async () => {
      if (!(await reviewResumeSuggestion(resumeID, suggestionID, accept))) {
        setError(true);
        return;
      }
      router.refresh();
    });
  }

  return (
    <div className="space-y-3">
      {suggestions.map((suggestion) => (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-secondary/50 p-3" key={suggestion.id}>
          <div>
            <p className="font-medium">{suggestion.value}</p>
            <p className="text-xs text-muted-foreground">
              {suggestion.kind} · {Math.round(suggestion.confidence * 100)}%
            </p>
          </div>
          {suggestion.status === "pending" ? (
            <div className="flex gap-2">
              <Button size="sm" disabled={busy} onClick={() => review(suggestion.id, true)}>{labels.accept}</Button>
              <Button size="sm" variant="outline" disabled={busy} onClick={() => review(suggestion.id, false)}>{labels.reject}</Button>
            </div>
          ) : (
            <span className="text-sm text-muted-foreground">{suggestion.status === "accepted" ? labels.accepted : labels.rejected}</span>
          )}
        </div>
      ))}
      {error && <p className="text-sm text-destructive" role="alert">{labels.failed}</p>}
    </div>
  );
}
