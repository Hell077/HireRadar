import Link from "next/link";

import { Button } from "@repo/ui/components/button";

import { AuthField } from "@/components/auth/auth-field";
import { AuthShell } from "@/components/auth/auth-shell";

export default function SignInPage() {
  return (
    <AuthShell
      title="Welcome back"
      description="Sign in to review new job matches and manage your search."
      footer={
        <>
          New to HireRadar?{" "}
          <Link className="font-medium text-primary hover:underline" href="/register">
            Create an account
          </Link>
        </>
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
        <AuthField
          id="password"
          label="Password"
          name="password"
          type="password"
          autoComplete="current-password"
          placeholder="Enter your password"
          required
          trailing={
            <Link className="text-sm text-primary hover:underline" href="/forgot-password">
              Forgot password?
            </Link>
          }
        />
        <Button className="h-11 w-full" type="submit">
          Sign in
        </Button>
      </form>
    </AuthShell>
  );
}
