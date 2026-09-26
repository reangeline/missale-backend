"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { AppShell } from "@/components/app-shell";
import { useAdmin } from "@/components/use-admin";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { api, describe, LANGUAGES, type Collection, type Release } from "@/lib/api";

export default function Home() {
  const email = useAdmin();
  const [collections, setCollections] = useState<Collection[]>([]);
  const [counts, setCounts] = useState<Record<string, Record<string, number>>>({});
  const [releases, setReleases] = useState<Release[]>([]);
  const [publishing, setPublishing] = useState(false);

  useEffect(() => {
    if (!email) return;
    (async () => {
      try {
        // Three requests, one after another: the API's Lambda has a small
        // concurrency limit, and a burst of parallel calls gets refused.
        setCollections((await api.collections()).collections);
        setCounts((await api.counts()).counts);
        setReleases((await api.releases()).releases);
      } catch (e) {
        toast.error(describe(e));
      }
    })();
  }, [email]);

  async function publish() {
    setPublishing(true);
    try {
      const r = await api.publish();
      toast.success(`Versão ${r.version} publicada: ${r.items} itens. O app atualiza na próxima abertura.`);
      setReleases((await api.releases()).releases);
    } catch (e) {
      toast.error(describe(e));
    } finally {
      setPublishing(false);
    }
  }

  if (!email) return null;
  const last = releases[0];

  return (
    <AppShell email={email}>
        <Card>
          <CardHeader>
            <CardTitle>Publicar no app</CardTitle>
            <CardDescription>
              O que você edita fica como rascunho. Publicar gera os arquivos que o app baixa: quem abrir o app depois disso
              recebe o conteúdo novo, sem atualizar pela App Store.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm text-muted-foreground">
              {last
                ? `Última publicação: versão ${last.version}, ${new Date(last.publishedAt).toLocaleString("pt-BR")}, por ${last.publishedBy}.`
                : "Nada publicado ainda: o app usa o conteúdo que veio com ele."}
            </p>
            <AlertDialog>
              <AlertDialogTrigger render={<Button disabled={publishing} />}>
                {publishing ? "Publicando…" : "Publicar agora"}
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Publicar todo o conteúdo?</AlertDialogTitle>
                  <AlertDialogDescription>
                    Todas as coleções, nos três idiomas, vão para o app como estão agora no painel.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancelar</AlertDialogCancel>
                  <AlertDialogAction onClick={publish}>Publicar</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </CardContent>
        </Card>

        <div>
          <h2 className="text-lg font-semibold">Conteúdo do app</h2>
          <p className="text-sm text-muted-foreground">
            Abra uma coleção para ver tudo o que o app tem, editar e criar itens novos. O menu à esquerda leva a qualquer uma delas.
          </p>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {collections.map((c) => (
            <Card key={c.key} className="h-full">
              <CardHeader>
                <CardTitle>{c.label}</CardTitle>
                <CardDescription>{c.description}</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-3">
                <div className="flex flex-wrap gap-2">
                  {LANGUAGES.map((l) => (
                    <Badge key={l.key} variant="secondary">
                      {l.key.toUpperCase()} · {counts[c.key]?.[l.key] ?? "…"}
                    </Badge>
                  ))}
                </div>
                <div className="flex gap-2">
                  <Link href={`/c/${c.key}`} className={buttonVariants({ size: "sm" })}>Ver e editar</Link>
                  <Link href={`/c/${c.key}?novo=1`} className={buttonVariants({ size: "sm", variant: "outline" })}>+ Novo</Link>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
    </AppShell>
  );
}
