"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Header } from "@/components/header";
import { api, type Collection } from "@/lib/api";

// The collection list rarely changes; fetched once per page load and shared.
let cached: Promise<Collection[]> | null = null;
function loadCollections() {
  cached ??= api.collections().then((r) => r.collections).catch((e) => {
    cached = null;
    throw e;
  });
  return cached;
}

/** Header, plus every collection always one click away in the sidebar. */
export function AppShell({ email, children }: { email: string; children: React.ReactNode }) {
  const pathname = usePathname();
  const [collections, setCollections] = useState<Collection[]>([]);
  useEffect(() => {
    let current = true;
    loadCollections().then((c) => current && setCollections(c)).catch(() => {});
    return () => {
      current = false;
    };
  }, []);

  const link = (href: string, label: string) => {
    const active = pathname === href;
    return (
      <Link key={href} href={href}
        className={`block rounded-md px-3 py-1.5 text-sm ${active ? "bg-primary text-primary-foreground" : "hover:bg-muted"}`}>
        {label}
      </Link>
    );
  };

  return (
    <>
      <Header email={email} />
      <div className="mx-auto flex w-full max-w-7xl flex-1 flex-col gap-4 p-4 md:flex-row">
        <nav className="shrink-0 md:w-56">
          <div className="rounded-lg border bg-background p-2 md:sticky md:top-4">
            {link("/", "Início e publicar")}
            <p className="px-3 pb-1 pt-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">Conteúdo</p>
            {collections.map((c) => link(`/c/${c.key}`, c.label))}
          </div>
        </nav>
        <main className="min-w-0 flex-1 space-y-4">{children}</main>
      </div>
    </>
  );
}
