import { AppHeader } from "@/components/app-header";
import { Skeleton } from "@repo/ui/components/skeleton";

export default function SettingsLoading() {
  return (
    <div className="min-h-screen">
      <AppHeader active="settings" />
      <main className="mx-auto w-full max-w-4xl px-4 py-8 sm:px-6 md:py-12">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="mt-4 h-10 w-56" />
        <Skeleton className="mt-3 h-4 w-full max-w-xl" />
        <div className="mt-8 space-y-6">
          {[1, 2, 3].map((item) => (
            <div key={item} className="rounded-xl border bg-card p-6">
              <Skeleton className="h-6 w-48" />
              <Skeleton className="mt-3 h-4 w-3/4" />
              <Skeleton className="mt-7 h-12 w-full" />
              <Skeleton className="mt-4 h-12 w-full" />
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
