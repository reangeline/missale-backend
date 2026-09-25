"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { signOut } from "@/lib/api";

export function Header({ email }: { email: string }) {
  const router = useRouter();
  return (
    <header className="border-b bg-background">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        <Link href="/" className="font-semibold tracking-tight">
          ✝︎ Missale <span className="text-muted-foreground font-normal">· Conteúdo</span>
        </Link>
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <span className="hidden sm:inline">{email}</span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              signOut();
              router.replace("/login");
            }}
          >
            Sair
          </Button>
        </div>
      </div>
    </header>
  );
}
