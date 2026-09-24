import { AppHeader } from "@/components/app-header";
import { JobListSkeleton } from "@/components/feedback/job-list-skeleton";
import { Skeleton } from "@repo/ui/components/skeleton";

export default function JobsLoading() {
  return (
    <div className="min-h-screen">
      <AppHeader active="jobs" />
      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="mt-4 h-10 w-72" />
        <Skeleton className="mt-3 h-4 w-full max-w-xl" />
        <Skeleton className="mt-8 h-28 w-full rounded-xl" />
        <div className="mt-6">
          <JobListSkeleton />
        </div>
      </main>
    </div>
  );
}
