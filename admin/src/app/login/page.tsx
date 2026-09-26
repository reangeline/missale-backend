"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, describe, saveSession } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState<string | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function finish(tokens: { accessToken?: string; refreshToken?: string }) {
    saveSession({ accessToken: tokens.accessToken!, refreshToken: tokens.refreshToken!, email: email.trim().toLowerCase() });
    router.replace("/");
  }

  async function onSignIn(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const s = await api.signIn(email, password);
      if (s.newPasswordNeeded) setChallenge(s.session!);
      else await finish(s);
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  async function onNewPassword(e: React.FormEvent) {
    e.preventDefault();
    if (newPassword !== confirm) return setError("As senhas não conferem.");
    if (newPassword.length < 12) return setError("Use pelo menos 12 caracteres, com maiúscula, minúscula, número e símbolo.");
    setBusy(true);
    setError(null);
    try {
      await finish(await api.newPassword(email.trim().toLowerCase(), challenge!, newPassword));
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>✝︎ Missale · Conteúdo</CardTitle>
          <CardDescription>
            {challenge ? "Primeiro acesso: escolha a sua senha." : "Entre com o seu e-mail de administrador."}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {!challenge ? (
            <form onSubmit={onSignIn} className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="email">E-mail</Label>
                <Input id="email" type="email" autoComplete="username" required value={email} onChange={(e) => setEmail(e.target.value)} />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="password">Senha</Label>
                <Input id="password" type="password" autoComplete="current-password" required value={password} onChange={(e) => setPassword(e.target.value)} />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" disabled={busy}>{busy ? "Entrando…" : "Entrar"}</Button>
            </form>
          ) : (
            <form onSubmit={onNewPassword} className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="new">Nova senha</Label>
                <Input id="new" type="password" autoComplete="new-password" required value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="confirm">Repita a nova senha</Label>
                <Input id="confirm" type="password" autoComplete="new-password" required value={confirm} onChange={(e) => setConfirm(e.target.value)} />
              </div>
              <p className="text-xs text-muted-foreground">Mínimo de 12 caracteres, com maiúscula, minúscula, número e símbolo.</p>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" disabled={busy}>{busy ? "Salvando…" : "Salvar e entrar"}</Button>
            </form>
          )}
        </CardContent>
      </Card>
    </main>
  );
}
