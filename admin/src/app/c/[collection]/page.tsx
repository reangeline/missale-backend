"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { AppShell } from "@/components/app-shell";
import { ItemEditor } from "@/components/item-editor";
import { useAdmin } from "@/components/use-admin";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api, describe, LANGUAGES, type Collection, type Item } from "@/lib/api";

export default function CollectionPage() {
  const email = useAdmin();
  const { collection: key } = useParams<{ collection: string }>();
  const [collection, setCollection] = useState<Collection | null>(null);
  const [lang, setLang] = useState<string>("pt");
  // null while a language is loading.
  const [items, setItems] = useState<Item[] | null>(null);
  const [query, setQuery] = useState("");
  // "+ Novo" on the overview opens the form straight away (?novo=1).
  const searchParams = useSearchParams();
  const [editing, setEditing] = useState<Item | "new" | null>(() => (searchParams.get("novo") ? "new" : null));

  useEffect(() => {
    if (!email) return;
    api.collections()
      .then(({ collections }) => setCollection(collections.find((c) => c.key === key) ?? null))
      .catch((e) => toast.error(describe(e)));
  }, [email, key]);

  // Bumped after a save or delete, to fetch the list again.
  const [revision, setRevision] = useState(0);
  const reload = useCallback(() => setRevision((r) => r + 1), []);

  useEffect(() => {
    if (!email) return;
    let current = true; // a newer language or revision supersedes this fetch
    api.list(key, lang)
      .then(({ items }) => current && setItems(items))
      .catch((e) => {
        if (!current) return;
        setItems([]);
        toast.error(describe(e));
      });
    return () => {
      current = false;
    };
  }, [email, key, lang, revision]);

  // The columns: the id, then the first two other fields (e.g. reference, text).
  const shown = useMemo(
    () => collection?.fields.filter((f) => f.key !== "id" && (f.type === "text" || f.type === "longtext")).slice(0, 2) ?? [],
    [collection],
  );
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!items || !q) return items ?? [];
    return items.filter((it) => JSON.stringify(it.data).toLowerCase().includes(q));
  }, [items, query]);

  if (!email) return null;

  return (
    <AppShell email={email}>
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <Link href="/" className="text-sm text-muted-foreground hover:underline">← Coleções</Link>
            <h1 className="text-2xl font-semibold tracking-tight">{collection?.label ?? key}</h1>
            <p className="text-sm text-muted-foreground">{collection?.description}</p>
          </div>
          <Button onClick={() => setEditing("new")} disabled={!collection}>Novo item</Button>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <Tabs value={lang} onValueChange={(v) => { setItems(null); setLang(String(v)); }}>
            <TabsList>
              {LANGUAGES.map((l) => (
                <TabsTrigger key={l.key} value={l.key}>{l.label}</TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
          <Input placeholder="Buscar em qualquer campo…" className="max-w-xs" value={query} onChange={(e) => setQuery(e.target.value)} />
        </div>

        <div className="rounded-lg border bg-background">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-12">#</TableHead>
                <TableHead>Identificador</TableHead>
                {shown.map((f) => <TableHead key={f.key}>{f.label}</TableHead>)}
                <TableHead className="hidden md:table-cell">Editado</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items === null ? (
                <TableRow><TableCell colSpan={4 + shown.length} className="text-muted-foreground">Carregando…</TableCell></TableRow>
              ) : filtered.length === 0 ? (
                <TableRow><TableCell colSpan={4 + shown.length} className="text-muted-foreground">
                  {items.length === 0 ? "Nenhum item neste idioma: o app usa a lista que veio com ele." : "Nada encontrado."}
                </TableCell></TableRow>
              ) : (
                filtered.map((it) => (
                  <TableRow key={it.id} className="cursor-pointer" onClick={() => setEditing(it)}>
                    <TableCell className="text-muted-foreground">{it.position + 1}</TableCell>
                    <TableCell className="font-mono text-xs">{it.id}</TableCell>
                    {shown.map((f) => (
                      <TableCell key={f.key} className="max-w-md truncate">
                        {typeof it.data[f.key] === "string" ? (it.data[f.key] as string) : ""}
                      </TableCell>
                    ))}
                    <TableCell className="hidden text-xs text-muted-foreground md:table-cell">
                      {new Date(it.updatedAt).toLocaleDateString("pt-BR")}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <p className="text-xs text-muted-foreground">
          {filtered.length} de {items?.length ?? 0} itens · clique numa linha para ver e editar · a ordem é a que o app usa.
        </p>

      {collection && (
        <ItemEditor collection={collection} lang={lang} item={editing} onClose={() => setEditing(null)} onSaved={reload} />
      )}
    </AppShell>
  );
}
