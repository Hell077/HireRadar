"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Plus, X } from "lucide-react";

import { Button } from "@repo/ui/components/button";
import { Input } from "@repo/ui/components/input";
import { normalizePositions } from "@/lib/profile/positions";

const suggestedRoles = [
  ["Go Developer", "goDeveloper"],
  ["Backend Engineer", "backendEngineer"],
  ["Frontend Engineer", "frontendEngineer"],
  ["React Developer", "reactDeveloper"],
  ["Full Stack Engineer", "fullstack"],
  ["Engineering Lead", "lead"],
] as const;

export function PositionSelector({
  initialPositions,
}: {
  initialPositions: string[];
}) {
  const t = useTranslations("jobFilters");
  const [positions, setPositions] = useState(() =>
    normalizePositions(initialPositions),
  );
  const [newPosition, setNewPosition] = useState("");

  function addPosition() {
    const additions = normalizePositions(newPosition.split(","));
    if (!additions.length) return;
    setPositions((current) => normalizePositions([...current, ...additions]));
    setNewPosition("");
  }

  function toggleSuggested(position: string) {
    setPositions((current) =>
      current.some(
        (item) => item.toLocaleLowerCase() === position.toLocaleLowerCase(),
      )
        ? current.filter(
            (item) => item.toLocaleLowerCase() !== position.toLocaleLowerCase(),
          )
        : normalizePositions([...current, position]),
    );
  }

  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium">{t("roles")}</legend>
      <p className="text-xs text-muted-foreground">{t("rolesHelp")}</p>
      <div className="flex flex-wrap gap-2">
        {suggestedRoles.map(([position, label]) => {
          const selected = positions.includes(position);
          return (
            <button
              key={position}
              type="button"
              aria-pressed={selected}
              onClick={() => toggleSuggested(position)}
              className={`rounded-full border px-3 py-2 text-sm font-medium transition-colors ${
                selected
                  ? "border-primary bg-accent text-accent-foreground"
                  : "border-border bg-card text-foreground hover:bg-accent/50"
              }`}
            >
              {t(label)}
            </button>
          );
        })}
      </div>
      <div className="flex gap-2">
        <Input
          value={newPosition}
          onChange={(event) => setNewPosition(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              addPosition();
            }
          }}
          placeholder={t("customRolePlaceholder")}
          aria-label={t("customRolePlaceholder")}
        />
        <Button
          type="button"
          variant="outline"
          onClick={addPosition}
          disabled={!newPosition.trim()}
        >
          <Plus aria-hidden="true" />
          {t("addRole")}
        </Button>
      </div>
      {positions.length ? (
        <ul className="flex flex-wrap gap-2" aria-label={t("selectedRoles")}>
          {positions.map((position) => (
            <li
              key={position}
              className="inline-flex items-center gap-2 rounded-full bg-secondary px-3 py-1.5 text-sm"
            >
              {position}
              <button
                type="button"
                onClick={() =>
                  setPositions((current) =>
                    current.filter((item) => item !== position),
                  )
                }
                className="text-muted-foreground hover:text-foreground"
                aria-label={t("removeRole", { role: position })}
              >
                <X className="size-3.5" aria-hidden="true" />
              </button>
              <input type="hidden" name="positions" value={position} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-xs text-muted-foreground">{t("noRolesSelected")}</p>
      )}
    </fieldset>
  );
}
