import Link from "next/link";
import { Bell, Radar } from "lucide-react";

const navigation = ["Profile", "Jobs", "Saved", "Settings"];

export function AppHeader() {
  return (
    <header className="border-border bg-card border-b">
      <div className="mx-auto flex h-18 max-w-[1440px] items-center justify-between px-5 md:px-14">
        <Link
          href="/"
          className="flex items-center gap-2.5"
          aria-label="HireRadar home"
        >
          <span className="bg-primary text-primary-foreground grid size-8 place-items-center rounded-[10px]">
            <Radar className="size-4.5" aria-hidden="true" />
          </span>
          <span className="text-[17px] font-bold tracking-tight">
            HireRadar
          </span>
        </Link>

        <nav
          className="hidden items-center gap-2 md:flex"
          aria-label="Primary navigation"
        >
          {navigation.map((item) => (
            <Link
              key={item}
              href={item === "Profile" ? "/" : `/${item.toLowerCase()}`}
              className={
                item === "Profile"
                  ? "bg-accent text-accent-foreground rounded-lg px-3.5 py-2 text-sm font-semibold"
                  : "text-muted-foreground hover:bg-secondary hover:text-foreground rounded-lg px-3.5 py-2 text-sm font-medium transition-colors"
              }
            >
              {item}
            </Link>
          ))}
        </nav>

        <div className="flex items-center gap-2.5">
          <button
            type="button"
            className="bg-secondary text-foreground grid size-9.5 place-items-center rounded-full"
            aria-label="Notifications"
          >
            <Bell className="size-4.5" aria-hidden="true" />
          </button>
          <div className="bg-primary text-primary-foreground grid size-9.5 place-items-center rounded-full text-xs font-bold">
            T
          </div>
        </div>
      </div>
    </header>
  );
}
