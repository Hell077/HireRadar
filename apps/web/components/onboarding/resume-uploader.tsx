"use client";

import { useState } from "react";

import { FileText, ShieldCheck, Upload } from "lucide-react";
import { Button } from "@repo/ui/components/button";
import { Card, CardContent } from "@repo/ui/components/card";

import {
  completeResumeUpload,
  createResumeUpload,
} from "@/app/resume-actions";

type Props = {
  labels: {
    drop: string;
    help: string;
    choose: string;
    privacy: string;
    uploading: string;
    uploaded: string;
    error: string;
  };
};

export function ResumeUploader({ labels }: Props) {
  const [status, setStatus] = useState<"idle" | "uploading" | "uploaded" | "error">("idle");

  async function upload(file?: File) {
    if (!file) return;
    setStatus("uploading");
    try {
      if (file.type !== "application/pdf" || file.size < 1 || file.size > 10 * 1024 * 1024) {
        throw new Error("invalid file");
      }
      const upload = await createResumeUpload(file.name, file.size);
      if (!upload) throw new Error("could not create upload URL");
      const put = await fetch(upload.upload_url, {
        method: "PUT",
        headers: { "Content-Type": "application/pdf" },
        body: file,
      });
      if (!put.ok || !(await completeResumeUpload(upload.resume.id))) {
        throw new Error("upload failed");
      }
      setStatus("uploaded");
    } catch {
      setStatus("error");
    }
  }

  const statusText = status === "uploading" ? labels.uploading : status === "uploaded" ? labels.uploaded : status === "error" ? labels.error : null;

  return (
    <Card>
      <CardContent>
        <label className="flex min-h-64 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed bg-secondary/40 px-6 py-10 text-center transition-colors hover:bg-accent/50">
          <input
            className="sr-only"
            type="file"
            accept="application/pdf,.pdf"
            disabled={status === "uploading"}
            onChange={(event) => upload(event.currentTarget.files?.[0])}
          />
          <span className="grid size-12 place-items-center rounded-xl bg-card text-primary shadow-sm">
            {status === "uploaded" ? <FileText className="size-5" aria-hidden="true" /> : <Upload className="size-5" aria-hidden="true" />}
          </span>
          <span className="mt-5 text-base font-semibold">{labels.drop}</span>
          <span className="mt-2 text-sm text-muted-foreground">{labels.help}</span>
          <Button className="mt-5" type="button" variant="outline">
            <FileText aria-hidden="true" />
            {labels.choose}
          </Button>
          {statusText && <span className="mt-3 text-sm text-muted-foreground" role="status">{statusText}</span>}
        </label>
        <div className="mt-5 flex items-start gap-3 rounded-lg bg-success-soft p-4 text-success">
          <ShieldCheck className="mt-0.5 size-5 shrink-0" aria-hidden="true" />
          <p className="text-xs leading-5">{labels.privacy}</p>
        </div>
      </CardContent>
    </Card>
  );
}
