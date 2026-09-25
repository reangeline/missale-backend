"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription,
  AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { api, describe, type Collection, type Item } from "@/lib/api";

type Props = {
  collection: Collection;
  lang: string;
  /** null = closed; "new" = a new item; otherwise the item being edited. */
  item: Item | "new" | null;
  onClose: () => void;
  onSaved: () => void;
};

/** A form drawn from the collection's fields, as the server declares them. */
export function ItemEditor(props: Props) {
  const { item, onClose } = props;
  return (
    <Dialog open={item !== null} onOpenChange={(open) => !open && onClose()}>
      {item !== null && (
        // Keyed by the item, so opening another one starts a fresh form.
        <EditorForm key={item === "new" ? "new" : item.id} {...props} item={item} />
      )}
    </Dialog>
  );
}

function EditorForm({ collection, lang, item, onClose, onSaved }: Props & { item: Item | "new" }) {
  const isNew = item === "new";
  const [values, setValues] = useState<Record<string, string>>(() =>
    isNew ? Object.fromEntries(collection.fields.map((f) => [f.key, ""])) : { ...item.data },
  );
  const [busy, setBusy] = useState(false);

  async function save() {
    const id = (values.id ?? "").trim();
    setBusy(true);
    try {
      await api.save(collection.key, lang, id, { ...values, id });
      toast.success("Salvo como rascunho. Publique para chegar ao app.");
      onSaved();
      onClose();
    } catch (e) {
      toast.error(describe(e));
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (isNew) return;
    setBusy(true);
    try {
      await api.remove(collection.key, lang, item.id);
      toast.success("Apagado. Publique para tirar do app.");
      onSaved();
      onClose();
    } catch (e) {
      toast.error(describe(e));
    } finally {
      setBusy(false);
    }
  }

  return (
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{isNew ? `Novo item · ${lang.toUpperCase()}` : `Editar · ${lang.toUpperCase()}`}</DialogTitle>
          <DialogDescription>{collection.label}</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          {collection.fields.map((f) => (
            <div key={f.key} className="grid gap-1.5">
              <Label htmlFor={f.key}>
                {f.label}
                {f.required && <span className="text-destructive"> *</span>}
              </Label>
              {f.type === "longtext" ? (
                <Textarea id={f.key} rows={5} value={values[f.key] ?? ""}
                  onChange={(e) => setValues({ ...values, [f.key]: e.target.value })} />
              ) : (
                <Input id={f.key} value={values[f.key] ?? ""} disabled={f.key === "id" && !isNew}
                  onChange={(e) => setValues({ ...values, [f.key]: e.target.value })} />
              )}
              {f.help && <p className="text-xs text-muted-foreground">{f.help}</p>}
            </div>
          ))}
        </div>
        <DialogFooter className="gap-2 sm:justify-between">
          {!isNew ? (
            <AlertDialog>
              <AlertDialogTrigger render={<Button variant="destructive" disabled={busy} />}>Apagar</AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Apagar este item?</AlertDialogTitle>
                  <AlertDialogDescription>Ele sai do app na próxima publicação.</AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Cancelar</AlertDialogCancel>
                  <AlertDialogAction onClick={remove}>Apagar</AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          ) : <span />}
          <div className="flex gap-2">
            <Button variant="outline" onClick={onClose} disabled={busy}>Cancelar</Button>
            <Button onClick={save} disabled={busy}>{busy ? "Salvando…" : "Salvar rascunho"}</Button>
          </div>
        </DialogFooter>
      </DialogContent>
  );
}
