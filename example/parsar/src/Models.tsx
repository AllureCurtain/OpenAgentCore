import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Cpu } from "lucide-react";
import { product, useModels, type ModelProfile } from "./lib/product";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./components/ui/dialog";
import { Field, Help, ErrorNotice, PageHeader } from "./components/shared";
import { EmptyState } from "./components/ui/empty-state";

export function Models() {
  const query = useModels();
  const cache = useQueryClient();
  const [editing, setEditing] = useState<ModelProfile | null>(null);
  const remove = useMutation({
    mutationFn: (id: string) => product(`models/${id}`, "DELETE"),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["models"] }),
  });
  return (
    <>
      <PageHeader title="模型">
        <Help>
          这里保存常用模型。模型需要已在 Core 部署中可用，API Key
          由部署管理。编辑不会改变已有 Agent；可在 Agent 配置中重新选择。
        </Help>
        <Button
          onClick={() =>
            setEditing({
              id: crypto.randomUUID(),
              name: "",
              model: "",
              revision: 0,
            })
          }
        >
          <Plus />
          添加模型
        </Button>
      </PageHeader>
      <ErrorNotice error={query.error || remove.error} />
      <div className="min-h-0 flex-1 overflow-auto p-6">
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {query.data?.map((model) => (
            <article
              key={model.id}
              className="rounded-xl border border-line p-5"
            >
              <div className="flex items-center gap-3">
                <Cpu className="h-5 w-5" />
                <h2 className="min-w-0 flex-1 truncate text-lg font-semibold">
                  {model.name}
                </h2>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`编辑 ${model.name}`}
                  onClick={() => setEditing(model)}
                >
                  <Pencil />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`移除 ${model.name}`}
                  disabled={remove.isPending}
                  onClick={() => {
                    if (confirm("移除这个常用模型配置？已有 Agent 不受影响。"))
                      remove.mutate(model.id);
                  }}
                >
                  <Trash2 />
                </Button>
              </div>
              <p className="mt-4 break-words text-base">{model.model}</p>
            </article>
          ))}
        </div>
        {!query.isPending && !query.data?.length && (
          <EmptyState title="添加常用模型，配置 Agent 时直接选用" />
        )}
      </div>
      {editing && (
        <ModelEditor value={editing} close={() => setEditing(null)} />
      )}
    </>
  );
}
function ModelEditor({
  value,
  close,
}: {
  value: ModelProfile;
  close: () => void;
}) {
  const [form, setForm] = useState(value);
  const cache = useQueryClient();
  const save = useMutation({
    mutationFn: () => product(`models/${form.id}`, "PUT", form),
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: ["models"] });
      close();
    },
  });
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) close();
      }}
    >
      <DialogContent aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>{value.revision ? "编辑模型" : "添加模型"}</DialogTitle>
        </DialogHeader>
        <form
          id="model-form"
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          <Field label="显示名称" id="model-name">
            <Input
              id="model-name"
              required
              value={form.name}
              maxLength={80}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
            />
          </Field>
          <Field label="模型 ID" id="model-id">
            <Input
              id="model-id"
              required
              value={form.model}
              maxLength={200}
              onChange={(e) => setForm({ ...form, model: e.target.value })}
              placeholder="例如 kimi-k2.6"
            />
          </Field>
          <ErrorNotice error={save.error} />
        </form>
        <DialogFooter>
          <Button type="submit" form="model-form" disabled={save.isPending}>
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
