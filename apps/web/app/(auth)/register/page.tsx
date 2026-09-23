import Link from "next/link";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default function RegisterPage() {
  return (
    <AuthShell
      title="Create your account"
      description="Start with your contact details. Your job profile comes next."
      footer={
        <>
          Already have an account?{" "}
          <Link className="font-medium text-primary hover:underline" href="/sign-in">
            Sign in
          </Link>
        </>
      }
    >
      <form className="space-y-5">
        <AuthField
          id="name"
          label="Full name"
          name="name"
          autoComplete="name"
          placeholder="Alex Morgan"
          required
        />
        <AuthField
          id="email"
          label="Email"
          name="email"
          type="email"
          autoComplete="email"
          placeholder="you@example.com"
          required
        />
        <AuthField
          id="password"
          label="Password"
          name="password"
          type="password"
          autoComplete="new-password"
          placeholder="At least 8 characters"
          minLength={8}
          required
        />
        <p className="text-xs leading-5 text-muted-foreground">
          By creating an account, you agree to the Terms of Service and Privacy
          Policy.
        </p>
        <Button className="h-11 w-full" type="submit">
          Create account
        </Button>
      </form>
    </AuthShell>
  );
}
