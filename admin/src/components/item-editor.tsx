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
import { api, describe, type Collection, type Field, type Item, type Value } from "@/lib/api";

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
  const [values, setValues] = useState<Record<string, Value>>(() =>
    isNew ? Object.fromEntries(collection.fields.map((f) => [f.key, emptyValue(f)])) : { ...item.data },
  );
  const [busy, setBusy] = useState(false);

  async function save() {
    const id = String(values.id ?? "").trim();
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
              <FieldInput field={f} value={values[f.key] ?? emptyValue(f)} disabled={f.key === "id" && !isNew}
                onChange={(v) => setValues({ ...values, [f.key]: v })} />
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

const ACCEPTED_IMAGES = ["image/jpeg", "image/png", "image/webp"];

/** An uploaded image: preview, send a new one, or remove it. */
function ImageInput({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [busy, setBusy] = useState(false);
  async function pick(file: File | undefined) {
    if (!file) return;
    if (!ACCEPTED_IMAGES.includes(file.type)) {
      toast.error("Use JPEG, PNG ou WebP.");
      return;
    }
    setBusy(true);
    try {
      onChange(await api.uploadImage(file));
      toast.success("Imagem enviada. Salve o rascunho e publique para chegar ao app.");
    } catch (e) {
      toast.error(describe(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="flex items-start gap-3">
      <div className="flex size-24 shrink-0 items-center justify-center overflow-hidden rounded-lg border bg-muted text-xs text-muted-foreground">
        {value ? (
          // eslint-disable-next-line @next/next/no-img-element -- a remote, already-optimized image; next/image needs the host configured
          <img src={value} alt="" className="size-full object-cover" />
        ) : (
          "sem imagem"
        )}
      </div>
      <div className="grid gap-2">
        <label className="inline-flex">
          <input type="file" accept={ACCEPTED_IMAGES.join(",")} className="sr-only" disabled={busy}
            onChange={(e) => { void pick(e.target.files?.[0]); e.target.value = ""; }} />
          <span className="cursor-pointer rounded-lg border px-2.5 py-1.5 text-sm hover:bg-muted">
            {busy ? "Enviando…" : value ? "Trocar imagem" : "Enviar imagem"}
          </span>
        </label>
        {value && (
          <Button type="button" variant="ghost" size="sm" className="justify-self-start" onClick={() => onChange("")}>
            Remover
          </Button>
        )}
      </div>
    </div>
  );
}

function emptyValue(f: Field): Value {
  return f.type === "paragraphs" || f.type === "items" ? [] : "";
}

/** One input per field type: a line, a block, a list of paragraphs, a list of records. */
function FieldInput({ field, value, disabled, onChange }: {
  field: Field; value: Value; disabled?: boolean; onChange: (v: Value) => void;
}) {
  if (field.type === "image") {
    return <ImageInput value={typeof value === "string" ? value : ""} onChange={onChange} />;
  }
  if (field.type === "paragraphs") {
    const list = Array.isArray(value) ? (value as string[]) : [];
    return (
      <div className="grid gap-2">
        {list.map((p, i) => (
          <div key={i} className="flex gap-2">
            <Textarea rows={4} value={p} aria-label={`${field.label} ${i + 1}`}
              onChange={(e) => onChange(list.map((x, j) => (j === i ? e.target.value : x)))} />
            <Button type="button" variant="ghost" size="sm" onClick={() => onChange(list.filter((_, j) => j !== i))}>
              Remover
            </Button>
          </div>
        ))}
        <Button type="button" variant="outline" size="sm" className="justify-self-start" onClick={() => onChange([...list, ""])}>
          + Parágrafo
        </Button>
      </div>
    );
  }
  if (field.type === "items") {
    const list = Array.isArray(value) ? (value as Record<string, string>[]) : [];
    const blank = () => Object.fromEntries((field.subfields ?? []).map((s) => [s.key, ""]));
    return (
      <div className="grid gap-3">
        {list.map((rec, i) => (
          <div key={i} className="grid gap-2 rounded-lg border p-3">
            {(field.subfields ?? []).map((sub) => (
              <div key={sub.key} className="grid gap-1">
                <Label className="text-xs">{sub.label}{sub.required && <span className="text-destructive"> *</span>}</Label>
                {sub.type === "longtext" ? (
                  <Textarea rows={3} value={rec[sub.key] ?? ""}
                    onChange={(e) => onChange(list.map((r, j) => (j === i ? { ...r, [sub.key]: e.target.value } : r)))} />
                ) : (
                  <Input value={rec[sub.key] ?? ""}
                    onChange={(e) => onChange(list.map((r, j) => (j === i ? { ...r, [sub.key]: e.target.value } : r)))} />
                )}
              </div>
            ))}
            <Button type="button" variant="ghost" size="sm" className="justify-self-end"
              onClick={() => onChange(list.filter((_, j) => j !== i))}>
              Remover
            </Button>
          </div>
        ))}
        <Button type="button" variant="outline" size="sm" className="justify-self-start" onClick={() => onChange([...list, blank()])}>
          + Adicionar
        </Button>
      </div>
    );
  }
  const text = typeof value === "string" ? value : "";
  return field.type === "longtext" ? (
    <Textarea id={field.key} rows={5} value={text} disabled={disabled} onChange={(e) => onChange(e.target.value)} />
  ) : (
    <Input id={field.key} value={text} disabled={disabled} pattern={field.pattern} onChange={(e) => onChange(e.target.value)} />
  );
}
