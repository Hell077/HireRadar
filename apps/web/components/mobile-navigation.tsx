import Link from "next/link";
import { BriefcaseBusiness, Bookmark, Settings, UserRound } from "lucide-react";

const items = [
  { label: "Jobs", href: "/jobs", icon: BriefcaseBusiness },
  { label: "Saved", href: "/saved", icon: Bookmark },
  { label: "Profile", href: "/", icon: UserRound, active: true },
  { label: "Settings", href: "/settings", icon: Settings },
];

export function MobileNavigation() {
  return (
    <nav
      className="border-border bg-card fixed inset-x-4 bottom-3 z-20 flex h-15 items-center gap-1 rounded-full border p-1.5 shadow-[0_8px_24px_color-mix(in_srgb,var(--foreground)_10%,transparent)] md:hidden"
      aria-label="Mobile navigation"
    >
      {items.map(({ label, href, icon: Icon, active }) => (
        <Link
          key={label}
          href={href}
          className={
            active
              ? "bg-accent text-accent-foreground flex h-full flex-1 flex-col items-center justify-center gap-0.5 rounded-full text-[10px] font-semibold"
              : "text-muted-foreground flex h-full flex-1 flex-col items-center justify-center gap-0.5 rounded-full text-[10px] font-medium"
          }
        >
          <Icon className="size-4.5" aria-hidden="true" />
          {label}
        </Link>
      ))}
    </nav>
  );
}
