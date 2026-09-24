"use client";

import { useState } from "react";

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

export function EditProfileDialog({ profile }: { profile: CandidateProfile }) {
  const t = useTranslations("profileEditor");
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
          <Field
            id="edit-country"
            name="country"
            label={t("country")}
            defaultValue={profile.country}
            maxLength={2}
          />
          <Field
            id="edit-city"
            name="city"
            label={t("city")}
            defaultValue={profile.city}
          />
          <Field
            id="edit-timezone"
            name="timezone"
            label={t("timezone")}
            defaultValue={profile.timezone}
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
          <Field
            id="edit-currency"
            name="currency"
            label={t("currency")}
            defaultValue={profile.desired_salary?.currency ?? "USD"}
            maxLength={3}
          />
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
  const [skills, setSkills] = useState(initialSkills.map(({ name }) => name));
  const [skill, setSkill] = useState("");

  function addSkill() {
    const value = skill.trim();
    if (value && !skills.includes(value))
      setSkills((items) => [...items, value]);
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
          <input className="sr-only" type="file" accept=".pdf,.doc,.docx" />
          <span className="grid size-11 place-items-center rounded-xl bg-card text-primary shadow-sm">
            <FileText className="size-5" aria-hidden="true" />
          </span>
          <span className="mt-4 text-sm font-semibold">
            {t("chooseResume")}
          </span>
          <span className="mt-1 text-xs text-muted-foreground">
            {t("fileHelp")}
          </span>
        </label>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{t("cancel")}</Button>
          </DialogClose>
          <DialogClose asChild>
            <Button>{t("upload")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
