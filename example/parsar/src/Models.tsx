import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2 } from "lucide-react";
import {
  product,
  useModels,
  useProviders,
  type ModelProfile,
  type ProviderProfile,
} from "./lib/product";
import { ModelEditor } from "./ModelEditor";
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
  const models = useModels();
  const providers = useProviders();
  const cache = useQueryClient();
  const [editing, setEditing] = useState<ModelProfile | null>(null);
  const [provider, setProvider] = useState<ProviderProfile | null>(null);
  const remove = useMutation({
    mutationFn: (path: string) => product(path, "DELETE"),
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: ["models"] });
      void cache.invalidateQueries({ queryKey: ["providers"] });
    },
  });
  return (
    <>
      <PageHeader title="模型">
        <Help>
          Provider 用于分组。模型连接和密钥仍由 Core 配置，Agent
          选择其中一个模型。修改模型只影响新会话。
        </Help>
        <Button
          onClick={() =>
            setProvider({ id: crypto.randomUUID(), name: "", revision: 0 })
          }
        >
          <Plus />
          添加 Provider
        </Button>
      </PageHeader>
      <ErrorNotice error={models.error || providers.error || remove.error} />
      <div className="min-h-0 flex-1 space-y-6 overflow-auto p-6">
        {models.data
          ?.filter((model) => !model.provider_id)
          .map((model) => (
            <div
              key={model.id}
              className="flex items-center justify-between gap-3"
            >
              <span>{model.name}</span>
              <Button variant="outline" onClick={() => setEditing(model)}>
                选择 Provider
              </Button>
            </div>
          ))}
        {providers.data?.map((group) => (
          <section
            key={group.id}
            aria-label={group.name}
            className="rounded-xl border border-line"
          >
            <div className="flex flex-wrap items-center gap-2 border-b border-line px-5 py-4">
              <h2 className="min-w-0 flex-1 break-words text-lg font-semibold">
                {group.name}
              </h2>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`编辑 Provider ${group.name}`}
                onClick={() => setProvider(group)}
              >
                <Pencil />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                aria-label={`移除 Provider ${group.name}`}
                disabled={remove.isPending}
                onClick={() => {
                  if (
                    confirm(`移除 Provider ${group.name}？请先移除其中的模型。`)
                  )
                    remove.mutate(`providers/${group.id}`);
                }}
              >
                <Trash2 />
              </Button>
              <Button
                variant="outline"
                onClick={() =>
                  setEditing({
                    id: crypto.randomUUID(),
                    provider_id: group.id,
                    name: "",
                    model: "",
                    revision: 0,
                  })
                }
              >
                <Plus />
                添加模型
              </Button>
            </div>
            <div className="divide-y divide-line">
              {models.data
                ?.filter((model) => model.provider_id === group.id)
                .map((model) => (
                  <div
                    key={model.id}
                    className="flex items-center gap-3 px-5 py-4"
                  >
                    <div className="min-w-0 flex-1">
                      <h3 className="truncate text-base font-medium">
                        {model.name}
                      </h3>
                      <p className="break-words text-base text-fg-muted">
                        {model.model}
                      </p>
                    </div>
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
                        if (confirm(`移除模型 ${model.name}？`))
                          remove.mutate(`models/${model.id}`);
                      }}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                ))}
            </div>
          </section>
        ))}
        {!providers.isPending && !providers.data?.length && (
          <EmptyState title="添加 Provider，再添加它的模型" />
        )}
      </div>
      {editing && (
        <ModelEditor value={editing} close={() => setEditing(null)} />
      )}
      {provider && (
        <ProviderEditor value={provider} close={() => setProvider(null)} />
      )}
    </>
  );
}
function ProviderEditor({
  value,
  close,
}: {
  value: ProviderProfile;
  close: () => void;
}) {
  const [name, setName] = useState(value.name);
  const cache = useQueryClient();
  const save = useMutation({
    mutationFn: () =>
      product(`providers/${value.id}`, "PUT", { ...value, name }),
    onSuccess: () => {
      void cache.invalidateQueries({ queryKey: ["providers"] });
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
          <DialogTitle>
            {value.revision ? "编辑 Provider" : "添加 Provider"}
          </DialogTitle>
        </DialogHeader>
        <form
          id="provider-form"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
          className="space-y-4"
        >
          <Field label="Provider 名称" id="provider-name">
            <Input
              id="provider-name"
              value={name}
              required
              maxLength={80}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如 OpenAI、MiniMax"
            />
          </Field>
          <ErrorNotice error={save.error} />
        </form>
        <DialogFooter>
          <Button type="submit" form="provider-form" disabled={save.isPending}>
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
