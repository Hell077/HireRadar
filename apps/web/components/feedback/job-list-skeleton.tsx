import { Skeleton } from "@repo/ui/components/skeleton";

export function JobListSkeleton({ count = 3 }: { count?: number }) {
  return (
    <div className="space-y-4">
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="rounded-xl border bg-card p-5 sm:p-6">
          <div className="flex gap-4">
            <Skeleton className="size-11 shrink-0 rounded-xl" />
            <div className="flex-1">
              <Skeleton className="h-5 w-2/5" />
              <Skeleton className="mt-2 h-4 w-1/4" />
              <div className="mt-5 flex gap-3">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-28" />
                <Skeleton className="h-4 w-16" />
              </div>
              <Skeleton className="mt-5 h-4 w-full" />
              <Skeleton className="mt-2 h-4 w-4/5" />
              <div className="mt-5 flex gap-2">
                <Skeleton className="h-7 w-16 rounded-full" />
                <Skeleton className="h-7 w-20 rounded-full" />
                <Skeleton className="h-7 w-24 rounded-full" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
