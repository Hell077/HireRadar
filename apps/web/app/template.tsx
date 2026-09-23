import type { ReactNode } from "react";

import { RouteTransition } from "@/components/motion/route-transition";

export default function Template({ children }: { children: ReactNode }) {
  return <RouteTransition>{children}</RouteTransition>;
}
