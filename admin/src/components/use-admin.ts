"use client";

import { useEffect, useSyncExternalStore } from "react";
import { useRouter } from "next/navigation";
import { storedSession } from "@/lib/api";

const noSubscription = () => () => {};

/**
 * The signed-in admin's email. Undefined while rendering on the server,
 * null when there is no session (and the page is on its way to /login).
 */
export function useAdmin() {
  const router = useRouter();
  const email = useSyncExternalStore(
    noSubscription,
    () => storedSession()?.email ?? null,
    () => undefined,
  );
  useEffect(() => {
    if (email === null) router.replace("/login");
  }, [email, router]);
  return email ?? null;
}
