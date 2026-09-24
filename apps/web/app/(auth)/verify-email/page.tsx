import Link from "next/link";

import { Button } from "@repo/ui/components/button";
import { AuthShell } from "@/components/auth/auth-shell";
import { verifyEmail } from "../actions";

export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string; error?: string; done?: string }>;
}) {
  const { token, error, done } = await searchParams;

  return (
    <AuthShell
      title={done === "1" ? "Email confirmed" : "Confirm your email"}
      description={
        done === "1"
          ? "Your email address is ready to use."
          : "Confirm your email address to complete registration."
      }
      footer={
        <Link
          className="font-medium text-primary hover:underline"
          href="/sign-in"
        >
          Return to sign in
        </Link>
      }
    >
      {done === "1" ? (
        <p className="text-sm text-success">
          You can now sign in to HireRadar.
        </p>
      ) : token ? (
        <form action={verifyEmail} className="space-y-5">
          <input type="hidden" name="token" value={token} />
          {error && (
            <p className="text-sm text-destructive">
              {error === "invalid"
                ? "This link has expired or was already used."
                : "Verification is unavailable. Please try again."}
            </p>
          )}
          <Button className="h-11 w-full" type="submit">
            Confirm email
          </Button>
        </form>
      ) : (
        <p className="text-sm text-destructive">
          This verification link is missing its token.
        </p>
      )}
    </AuthShell>
  );
}
