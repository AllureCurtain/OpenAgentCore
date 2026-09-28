import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  product,
  useTemplates,
  useRuntimes,
  type AgentTemplate,
  type AgentInstance,
} from "./lib/product";
import { ConfigurationFields, emptyConfiguration } from "./ConfigurationFields";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import { Select, SelectOption } from "./components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./components/ui/dialog";
import { ErrorNotice, Field, Help } from "./components/shared";
export function InstanceEditor({
  template,
  instance,
  close,
}: {
  template?: AgentTemplate;
  instance?: AgentInstance;
  close: () => void;
}) {
  const cache = useQueryClient();
  const templates = useTemplates();
  const runtimes = useRuntimes();
  const [form, setForm] = useState(() => ({
    ...(instance || template || emptyConfiguration),
    id: instance?.id || crypto.randomUUID(),
    name: instance?.name || template?.name || "",
    template_id: instance?.template_id || template?.id || "",
    runtime_id: instance?.runtime_id || "",
    revision: instance?.revision || 0,
  }));
  const save = useMutation({
    mutationFn: () =>
      product<AgentInstance>(`instances/${form.id}`, "PUT", form),
    onSuccess: (agent) => {
      void cache.invalidateQueries({ queryKey: ["instances"] });
      void cache.invalidateQueries({ queryKey: ["instance", agent.id] });
      close();
      location.hash = `/agents/${agent.id}`;
    },
  });
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !save.isPending) close();
      }}
    >
      <DialogContent
        aria-describedby={undefined}
        className="max-h-[90dvh] overflow-y-auto sm:max-w-xl"
      >
        <DialogHeader>
          <DialogTitle>{instance ? "编辑 Agent" : "创建 Agent"}</DialogTitle>
        </DialogHeader>
        <form
          id="instance-form"
          className="space-y-5"
          onSubmit={(e) => {
            e.preventDefault();
            save.mutate();
          }}
        >
          {!instance && (
            <Field label="Agent 模板" id="instance-template">
              <Select
                id="instance-template"
                value={form.template_id}
                onValueChange={(id) => {
                  const selected = templates.data?.find((t) => t.id === id);
                  if (selected)
                    setForm({
                      ...form,
                      ...selected,
                      id: form.id,
                      name: selected.name,
                      template_id: id,
                      revision: 0,
                    });
                }}
              >
                <SelectOption value="">选择模板</SelectOption>
                {templates.data?.map((t) => (
                  <SelectOption key={t.id} value={t.id}>
                    {t.name}
                  </SelectOption>
                ))}
              </Select>
              {!templates.isPending && !templates.data?.length && (
                <a
                  href="#/templates"
                  onClick={close}
                  className="inline-block underline"
                >
                  前往创建模板
                </a>
              )}
            </Field>
          )}
          <Field label="Agent 名称" id="instance-name">
            <Input
              id="instance-name"
              required
              maxLength={80}
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
            />
          </Field>
          <Field label="运行时" id="instance-runtime">
            <Select
              id="instance-runtime"
              value={form.runtime_id}
              onValueChange={(runtime_id) => setForm({ ...form, runtime_id })}
            >
              <SelectOption value="">选择运行时</SelectOption>
              {runtimes.data?.map((runtime) => (
                <SelectOption key={runtime.id} value={runtime.id}>
                  {runtime.name}
                </SelectOption>
              ))}
            </Select>
            {!runtimes.isPending && !runtimes.data?.length && (
              <a
                href="#/runtimes"
                onClick={close}
                className="inline-block underline"
              >
                前往添加运行时
              </a>
            )}
          </Field>
          {instance && (
            <ConfigurationFields
              value={form}
              change={(configuration) => setForm({ ...form, ...configuration })}
            />
          )}
          <ErrorNotice
            error={save.error || templates.error || runtimes.error}
          />
        </form>
        <DialogFooter>
          <Help>
            {instance
              ? "保存会更新此 Agent 的配置，不修改来源模板。"
              : "模板配置会复制到这个 Agent，运行时决定它在哪里工作。创建仅保存配置，不会启动模型执行。"}
          </Help>
          <Button
            type="submit"
            form="instance-form"
            disabled={
              save.isPending ||
              !form.template_id ||
              !form.runtime_id ||
              !form.model_id
            }
          >
            {save.isPending ? "保存中…" : instance ? "保存" : "创建"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
