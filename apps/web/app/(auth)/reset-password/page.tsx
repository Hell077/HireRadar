import Link from "next/link";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default function ResetPasswordPage() {
  return (
    <AuthShell
      title="Choose a new password"
      description="Use at least 8 characters and avoid a password used elsewhere."
      footer={
        <Link className="font-medium text-primary hover:underline" href="/sign-in">
          Return to sign in
        </Link>
      }
    >
      <form className="space-y-5">
        <AuthField
          id="password"
          label="New password"
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder="Enter a new password"
          minLength={8}
          required
        />
        <AuthField
          id="confirm-password"
          label="Confirm password"
          name="confirmPassword"
          type="password"
          autoComplete="new-password"
          placeholder="Repeat your new password"
          minLength={8}
          required
        />
        <Button className="h-11 w-full" type="submit">
          Update password
        </Button>
      </form>
    </AuthShell>
  );
}
