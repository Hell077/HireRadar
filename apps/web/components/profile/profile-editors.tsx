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

export function EditProfileDialog() {
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
        <form className="grid gap-5 py-2 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <Field
              id="edit-name"
              label={t("fullName")}
              defaultValue="Timur K."
            />
          </div>
          <Field
            id="edit-role"
            label={t("role")}
            defaultValue="Golang & Frontend Developer"
          />
          <Field
            id="edit-experience"
            label={t("experience")}
            type="number"
            defaultValue="5"
          />
          <Field
            id="edit-location"
            label={t("location")}
            defaultValue="Kazakhstan"
          />
          <Field
            id="edit-timezone"
            label={t("timezone")}
            defaultValue="UTC+5"
          />
          <Field
            id="edit-salary"
            label={t("salary")}
            placeholder={t("salaryPlaceholder")}
          />
          <Field id="edit-currency" label={t("currency")} defaultValue="USD" />
        </form>
        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline">{t("cancel")}</Button>
          </DialogClose>
          <DialogClose asChild>
            <Button>{t("save")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const initialSkills = [
  "Go",
  "React",
  "Next.js",
  "TypeScript",
  "PostgreSQL",
  "Docker",
];

export function EditSkillsSheet() {
  const t = useTranslations("profileEditor");
  const [skills, setSkills] = useState(initialSkills);
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
        <SheetFooter>
          <SheetClose asChild>
            <Button variant="outline">{t("cancel")}</Button>
          </SheetClose>
          <SheetClose asChild>
            <Button>{t("saveSkills")}</Button>
          </SheetClose>
        </SheetFooter>
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
