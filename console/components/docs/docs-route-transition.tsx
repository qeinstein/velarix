"use client";

import type { ReactNode } from "react";

import { AnimatePresence, motion } from "motion/react";
import { usePathname } from "next/navigation";

const mechanicalEase: [number, number, number, number] = [0.16, 1, 0.3, 1];

export function DocsRouteTransition({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  return (
    <AnimatePresence mode="wait">
      <motion.div
        key={pathname}
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        exit={{ opacity: 0, y: -8 }}
        transition={{ duration: 0.24, ease: mechanicalEase }}
      >
        {children}
      </motion.div>
    </AnimatePresence>
  );
}
