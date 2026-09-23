import Link from "next/link";

import { ArrowLeft } from "lucide-react";
import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default function ForgotPasswordPage() {
  return (
    <AuthShell
      title="Reset your password"
      description="Enter your account email and we’ll send you a reset link."
      footer={
        <Link
          className="inline-flex items-center gap-2 font-medium text-primary hover:underline"
          href="/sign-in"
        >
          <ArrowLeft className="size-4" />
          Back to sign in
        </Link>
      }
    >
      <form className="space-y-5">
        <AuthField
          id="email"
          label="Email"
          name="email"
          type="email"
          autoComplete="email"
          placeholder="you@example.com"
          required
        />
        <Button className="h-11 w-full" type="submit">
          Send reset link
        </Button>
      </form>
    </AuthShell>
  );
}
