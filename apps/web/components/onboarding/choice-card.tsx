import type { ReactNode } from "react";

type ChoiceCardProps = {
  checked?: boolean;
  description?: string;
  icon?: ReactNode;
  label: string;
  name: string;
};

export function ChoiceCard({
  checked = false,
  description,
  icon,
  label,
  name,
}: ChoiceCardProps) {
  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-xl border bg-card p-4 transition-colors hover:bg-accent/40 has-checked:border-primary has-checked:bg-accent/60">
      <input
        className="mt-1 size-4 accent-primary"
        type="checkbox"
        name={name}
        defaultChecked={checked}
      />
      {icon ? (
        <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-secondary text-primary">
          {icon}
        </span>
      ) : null}
      <span className="min-w-0">
        <span className="block text-sm font-semibold">{label}</span>
        {description ? (
          <span className="mt-1 block text-xs leading-5 text-muted-foreground">
            {description}
          </span>
        ) : null}
      </span>
    </label>
  );
}
