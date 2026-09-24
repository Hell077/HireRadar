import { AppHeader } from "@/components/app-header";
import { JobListSkeleton } from "@/components/feedback/job-list-skeleton";
import { Skeleton } from "@repo/ui/components/skeleton";

export default function SavedLoading() {
  return (
    <div className="min-h-screen">
      <AppHeader active="saved" />
      <main className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6 md:py-12">
        <Skeleton className="h-4 w-28" />
        <Skeleton className="mt-4 h-10 w-64" />
        <Skeleton className="mt-3 h-4 w-96 max-w-full" />
        <div className="mt-8">
          <JobListSkeleton count={2} />
        </div>
      </main>
    </div>
  );
}
