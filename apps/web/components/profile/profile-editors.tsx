"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

import { FileText, Pencil, Plus, Upload, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@repo/ui/components/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@repo/ui/components/dialog";
import { Input } from "@repo/ui/components/input";
import { Label } from "@repo/ui/components/label";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@repo/ui/components/sheet";

import { updateProfile, updateSkills } from "@/app/profile-actions";
import type { CandidateProfile, CandidateSkill } from "@/lib/api/server";
import { completeResumeUpload, createResumeUpload } from "@/app/resume-actions";

function Field({
  label,
  id,
  ...props
}: React.ComponentProps<typeof Input> & { label: string }) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} className="h-11" {...props} />
    </div>
  );
}

const skillAliases: Record<string, string> = {
  go: "Go",
  golang: "Go",
  js: "JavaScript",
  javascript: "JavaScript",
  ts: "TypeScript",
  typescript: "TypeScript",
  reactjs: "React",
  "react.js": "React",
  nextjs: "Next.js",
  "next.js": "Next.js",
  postgres: "PostgreSQL",
  postgresql: "PostgreSQL",
};

function canonicalSkill(value: string) {
  const trimmed = value.trim();
  return skillAliases[trimmed.toLocaleLowerCase()] ?? trimmed;
}

export function EditProfileDialog({ profile }: { profile: CandidateProfile }) {
  const t = useTranslations("profileEditor");
  const [country, setCountry] = useState(profile.country || "KZ");
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="lg">
          <Pencil aria-hidden="true" />
          {t("editProfile")}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("profileTitle")}</DialogTitle>
          <DialogDescription>{t("profileDescription")}</DialogDescription>
        </DialogHeader>
        <form action={updateProfile} className="grid gap-5 py-2 sm:grid-cols-2">
          <Field
            id="edit-first-name"
            name="firstName"
            label={t("firstName")}
            defaultValue={profile.first_name}
          />
          <Field
            id="edit-last-name"
            name="lastName"
            label={t("lastName")}
            defaultValue={profile.last_name}
          />
          <Field
            id="edit-seniority"
            name="seniority"
            label={t("seniority")}
            defaultValue={profile.seniority}
          />
          <Field
            id="edit-experience"
            name="experience"
            label={t("experience")}
            type="number"
            min={0}
            max={70}
            defaultValue={profile.experience_years}
          />
          <div className="space-y-2">
            <Label htmlFor="edit-country">{t("country")}</Label>
            <select
              id="edit-country"
              name="country"
              value={country}
              onChange={(event) => setCountry(event.target.value)}
              className="h-11 w-full rounded-md border border-input bg-background px-3 text-sm"
            >
              {countries.map(([code, label]) => (
                <option key={code} value={code}>
                  {label}
                </option>
              ))}
            </select>
            <p className="text-xs text-muted-foreground">{t("locationHelp")}</p>
          </div>
          <Field
            id="edit-city"
            name="city"
            label={t("city")}
            defaultValue={profile.city}
          />
          <input
            type="hidden"
            name="timezone"
            value={timezones[country] ?? profile.timezone ?? "UTC"}
          />
          <Field
            id="edit-salary"
            name="salary"
            label={t("salary")}
            type="number"
            min={0}
            defaultValue={profile.desired_salary?.amount}
            placeholder={t("salaryPlaceholder")}
          />
          <div className="space-y-2">
            <Label htmlFor="edit-currency">{t("currency")}</Label>
            <select
              id="edit-currency"
              name="currency"
              defaultValue={profile.desired_salary?.currency ?? "USD"}
              className="h-11 w-full rounded-md border border-input bg-background px-3 text-sm"
            >
              {["USD", "EUR", "KZT", "GBP"].map((currency) => (
                <option key={currency} value={currency}>
                  {currency}
                </option>
              ))}
            </select>
          </div>
          <DialogFooter className="sm:col-span-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                {t("cancel")}
              </Button>
            </DialogClose>
            <Button type="submit">{t("save")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function EditSkillsSheet({
  initialSkills,
}: {
  initialSkills: CandidateSkill[];
}) {
  const t = useTranslations("profileEditor");
  const [skills, setSkills] = useState(() =>
    initialSkills
      .map(({ name }) => canonicalSkill(name))
      .filter(
        (name, index, values) =>
          values.findIndex(
            (candidate) =>
              candidate.toLocaleLowerCase() === name.toLocaleLowerCase(),
          ) === index,
      ),
  );
  const [skill, setSkill] = useState("");
  const [skillNotice, setSkillNotice] = useState("");

  function addSkill() {
    const value = canonicalSkill(skill);
    if (!value) return;
    if (
      skills.some(
        (item) => item.toLocaleLowerCase() === value.toLocaleLowerCase(),
      )
    ) {
      setSkillNotice(t("skillDuplicate", { skill: value }));
      return;
    }
    setSkills((items) => [...items, value]);
    setSkillNotice(
      value !== skill.trim() ? t("skillNormalized", { skill: value }) : "",
    );
    setSkill("");
  }

  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button variant="ghost" size="sm" className="text-primary">
          <Pencil aria-hidden="true" />
          {t("edit")}
        </Button>
      </SheetTrigger>
      <SheetContent side="right">
        <SheetHeader>
          <SheetTitle>{t("skillsTitle")}</SheetTitle>
          <SheetDescription>{t("skillsDescription")}</SheetDescription>
        </SheetHeader>
        <form action={updateSkills} className="flex min-h-0 flex-1 flex-col">
          <div className="flex gap-2 py-3">
            <Input
              value={skill}
              onFocus={() => setSkillNotice("")}
              onChange={(event) => setSkill(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  addSkill();
                }
              }}
              placeholder={t("skillPlaceholder")}
            />
            <Button
              type="button"
              size="icon"
              onClick={addSkill}
              aria-label={t("addSkill")}
            >
              <Plus aria-hidden="true" />
            </Button>
          </div>
          {skillNotice ? (
            <p className="pb-3 text-xs text-muted-foreground">{skillNotice}</p>
          ) : null}
          <div className="flex flex-wrap gap-2">
            {skills.map((item) => (
              <span
                key={item}
                className="inline-flex items-center gap-2 rounded-full bg-secondary px-3 py-2 text-sm font-medium"
              >
                <input type="hidden" name="skills" value={item} />
                {item}
                <button
                  type="button"
                  onClick={() =>
                    setSkills((values) =>
                      values.filter((value) => value !== item),
                    )
                  }
                  className="rounded-full text-muted-foreground hover:text-foreground"
                  aria-label={t("removeSkill", { skill: item })}
                >
                  <X className="size-3.5" aria-hidden="true" />
                </button>
              </span>
            ))}
          </div>
          <SheetFooter className="mt-auto">
            <SheetClose asChild>
              <Button variant="outline">{t("cancel")}</Button>
            </SheetClose>
            <Button type="submit">{t("saveSkills")}</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

export function ReplaceResumeDialog() {
  const t = useTranslations("profileEditor");
  const router = useRouter();
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState(false);

  async function upload() {
    if (
      !file ||
      file.type !== "application/pdf" ||
      file.size > 10 * 1024 * 1024
    ) {
      setError(true);
      return;
    }
    setUploading(true);
    setError(false);
    try {
      const result = await createResumeUpload(file.name, file.size);
      if (!result) throw new Error("upload URL unavailable");
      const response = await fetch(result.upload_url, {
        method: "PUT",
        headers: { "Content-Type": "application/pdf" },
        body: file,
      });
      if (!response.ok || !(await completeResumeUpload(result.resume.id))) {
        throw new Error("upload failed");
      }
      router.refresh();
    } catch {
      setError(true);
    } finally {
      setUploading(false);
    }
  }
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Upload aria-hidden="true" />
          {t("replace")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("resumeTitle")}</DialogTitle>
          <DialogDescription>{t("resumeDescription")}</DialogDescription>
        </DialogHeader>
        <label className="flex min-h-48 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed bg-secondary/40 p-6 text-center hover:bg-accent/50">
          <input
            className="sr-only"
            type="file"
            accept="application/pdf,.pdf"
            onChange={(event) =>
              setFile(event.currentTarget.files?.[0] ?? null)
            }
          />
          <span className="grid size-11 place-items-center rounded-xl bg-card text-primary shadow-sm">
            <FileText className="size-5" aria-hidden="true" />
          </span>
          <span className="mt-4 text-sm font-semibold">
            {file?.name ?? t("chooseResume")}
          </span>
          <span className="mt-1 text-xs text-muted-foreground">
            {t("fileHelp")}
          </span>
        </label>
        {error ? (
          <p className="text-sm text-destructive">{t("uploadError")}</p>
        ) : null}
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{t("cancel")}</Button>
          </DialogClose>
          <Button type="button" onClick={upload} disabled={!file || uploading}>
            {uploading ? t("uploading") : t("upload")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
const countries = [
  ["KZ", "Казахстан / Kazakhstan"],
  ["KG", "Кыргызстан / Kyrgyzstan"],
  ["UZ", "Узбекистан / Uzbekistan"],
  ["AZ", "Азербайджан / Azerbaijan"],
  ["AM", "Армения / Armenia"],
  ["GE", "Грузия / Georgia"],
  ["UA", "Украина / Ukraine"],
  ["RU", "Россия / Russia"],
] as const;

const timezones: Record<string, string> = {
  KZ: "Asia/Almaty",
  KG: "Asia/Bishkek",
  UZ: "Asia/Tashkent",
  AZ: "Asia/Baku",
  AM: "Asia/Yerevan",
  GE: "Asia/Tbilisi",
  UA: "Europe/Kyiv",
  RU: "Europe/Moscow",
};
