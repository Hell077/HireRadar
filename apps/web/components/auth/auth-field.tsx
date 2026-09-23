import type { ComponentProps } from "react";

import { Input } from "@repo/ui/components/input";
import { Label } from "@repo/ui/components/label";

type AuthFieldProps = ComponentProps<typeof Input> & {
  label: string;
  trailing?: React.ReactNode;
};

export function AuthField({ id, label, trailing, ...props }: AuthFieldProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-4">
        <Label htmlFor={id}>{label}</Label>
        {trailing}
      </div>
      <Input id={id} className="h-11" {...props} />
    </div>
  );
}
