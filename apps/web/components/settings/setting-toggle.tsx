"use client";

import { useId, useState } from "react";

type SettingToggleProps = {
  defaultChecked?: boolean;
  description: string;
  label: string;
};

export function SettingToggle({
  defaultChecked = false,
  description,
  label,
}: SettingToggleProps) {
  const id = useId();
  const [checked, setChecked] = useState(defaultChecked);

  return (
    <div className="flex items-start justify-between gap-5 py-4 first:pt-0 last:pb-0">
      <div>
        <label htmlFor={id} className="cursor-pointer text-sm font-semibold">
          {label}
        </label>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          {description}
        </p>
      </div>
      <button
        id={id}
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => setChecked((value) => !value)}
        className={`relative mt-0.5 h-6 w-11 shrink-0 rounded-full p-0.5 transition-colors ${checked ? "bg-primary" : "bg-input"}`}
      >
        <span
          className={`block size-5 rounded-full bg-card shadow-sm transition-transform ${checked ? "translate-x-5" : "translate-x-0"}`}
        />
      </button>
    </div>
  );
}
