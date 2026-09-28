import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { product, useProviders, type ModelProfile } from "./lib/product";
import { Button } from "./components/ui/button";
import { Select, SelectOption } from "./components/ui/select";
import { Input } from "./components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "./components/ui/dialog";
import { Field, ErrorNotice } from "./components/shared";

export function ModelEditor({
  value,
  close,
}: {
  value: ModelProfile;
  close: () => void;
}) {
  const [form, setForm] = useState(value);
  const cache = useQueryClient();
  const providers = useProviders();
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
          <Field label="Provider" id="model-provider">
            <Select
              id="model-provider"
              value={form.provider_id || ""}
              onValueChange={(provider_id) => setForm({ ...form, provider_id })}
            >
              <SelectOption value="">选择 Provider</SelectOption>
              {providers.data?.map((provider) => (
                <SelectOption key={provider.id} value={provider.id}>
                  {provider.name}
                </SelectOption>
              ))}
            </Select>
          </Field>
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
          <Button
            type="submit"
            form="model-form"
            disabled={save.isPending || !form.provider_id}
          >
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
