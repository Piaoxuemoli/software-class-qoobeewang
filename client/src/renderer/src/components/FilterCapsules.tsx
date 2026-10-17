import { useState } from 'react';
import type { FileTypeCode } from '@/shared/protocol';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Checkbox } from '@/components/ui/checkbox';
import { Textarea } from '@/components/ui/textarea';
import { useJobStore } from '../stores/job';
import { cn } from '@/lib/utils';

/** ui-design.md §5：筛选条件 = 一排胶囊按钮，点击展开为 Popover 编辑。 */

const ALL_TYPES: { code: FileTypeCode; label: string }[] = [
  { code: 'f', label: '普通文件' },
  { code: 'd', label: '目录' },
  { code: 'l', label: '软链接' },
  { code: 'c', label: '字符设备' },
  { code: 'b', label: '块设备' },
  { code: 's', label: '套接字' },
  { code: 'p', label: '管道' },
];

function Capsule({
  label,
  active,
  summary,
  onClear,
  children,
}: {
  label: string;
  /** 该维度是否已设置（显示白底黑字 + 摘要 + 清除按钮）。 */
  active: boolean;
  summary: string | null;
  onClear: () => void;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <div className="flex items-center">
      <Popover>
        <PopoverTrigger asChild>
          <button
            type="button"
            className={cn(
              'rounded-full border px-3 py-1.5 text-[12px] transition-colors',
              active
                ? 'border-accent bg-accent text-black'
                : 'border-border bg-surface-1 text-text-2 hover:bg-surface-2',
            )}
          >
            {label}
            {active && summary && <span className="ml-1 opacity-70">· {summary}</span>}
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-80">{children}</PopoverContent>
      </Popover>
      {active && (
        <button
          type="button"
          aria-label={`清除${label}筛选`}
          className="-ml-4 rounded-full bg-accent px-2 py-1.5 text-[12px] leading-none text-black hover:text-danger"
          onClick={onClear}
        >
          ×
        </button>
      )}
    </div>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <Label className="flex flex-col gap-1 text-[12px] text-text-2">
      <span>{label}</span>
      {children}
    </Label>
  );
}

/** 逗号/换行分隔文本 ↔ 字符串数组（排除用户、名称、目录）。 */
function ListInput({
  value,
  onChange,
  placeholder,
  multiline,
}: {
  value: string[] | undefined;
  onChange: (v: string[] | undefined) => void;
  placeholder?: string;
  multiline?: boolean;
}): React.JSX.Element {
  const [text, setText] = useState((value ?? []).join(multiline ? '\n' : ', '));
  const commit = (t: string): void => {
    setText(t);
    const items = t
      .split(multiline ? /\n/ : /[,，\n]/)
      .map((s) => s.trim())
      .filter((s) => s !== '');
    onChange(items.length === 0 ? undefined : items);
  };
  return multiline ? (
    <Textarea
      value={text}
      placeholder={placeholder}
      rows={3}
      className="text-[12px]"
      onChange={(e) => commit(e.target.value)}
    />
  ) : (
    <Input
      value={text}
      placeholder={placeholder}
      className="h-8 text-[12px]"
      onChange={(e) => commit(e.target.value)}
    />
  );
}

function numOrUndef(v: string): number | undefined {
  if (v.trim() === '') return undefined;
  const n = Number(v);
  return Number.isNaN(n) ? undefined : n;
}

/** 六个筛选维度胶囊；所有值原样进入 BackupRequest.filters，合法性由后端判定。 */
export function FilterCapsules(): React.JSX.Element {
  const filters = useJobStore((s) => s.form.filters);
  const setFilters = useJobStore((s) => s.setFilters);

  const types = filters.types;
  const typeSet = new Set(types ?? ALL_TYPES.map((t) => t.code));
  const toggleType = (code: FileTypeCode): void => {
    const next = new Set(typeSet);
    if (next.has(code)) next.delete(code);
    else next.add(code);
    setFilters({ types: next.size === ALL_TYPES.length ? undefined : [...next] });
  };

  const times = [filters.mtimeFrom, filters.mtimeTo, filters.ctimeFrom, filters.ctimeTo];
  const sizes = [filters.sizeMin, filters.sizeMax];
  const who = [filters.excludeUsers, filters.excludeGroups];

  return (
    <div className="flex flex-wrap items-center gap-2" data-testid="filter-capsules">
      <Capsule
        label="类型"
        active={types !== undefined}
        summary={types ? `${types.length} 项` : null}
        onClear={() => setFilters({ types: undefined })}
      >
        <div className="grid grid-cols-2 gap-2">
          {ALL_TYPES.map((t) => (
            <label key={t.code} className="flex items-center gap-2 text-[13px]">
              <Checkbox checked={typeSet.has(t.code)} onCheckedChange={() => toggleType(t.code)} />
              {t.label}
            </label>
          ))}
        </div>
      </Capsule>

      <Capsule
        label="时间"
        active={times.some((v) => v !== undefined)}
        summary={times.filter((v) => v !== undefined).length + ' 项'}
        onClear={() =>
          setFilters({
            mtimeFrom: undefined,
            mtimeTo: undefined,
            ctimeFrom: undefined,
            ctimeTo: undefined,
          })
        }
      >
        <div className="flex flex-col gap-3">
          <div className="grid grid-cols-2 gap-2">
            <Field label="修改时间起">
              <Input
                placeholder="YYYY-MM-DD"
                className="h-8 text-[12px]"
                value={filters.mtimeFrom ?? ''}
                onChange={(e) => setFilters({ mtimeFrom: e.target.value || undefined })}
              />
            </Field>
            <Field label="修改时间止">
              <Input
                placeholder="YYYY-MM-DD"
                className="h-8 text-[12px]"
                value={filters.mtimeTo ?? ''}
                onChange={(e) => setFilters({ mtimeTo: e.target.value || undefined })}
              />
            </Field>
          </div>
          <div className="grid grid-cols-2 gap-2">
            <Field label="创建时间起">
              <Input
                placeholder="YYYY-MM-DD"
                className="h-8 text-[12px]"
                value={filters.ctimeFrom ?? ''}
                onChange={(e) => setFilters({ ctimeFrom: e.target.value || undefined })}
              />
            </Field>
            <Field label="创建时间止">
              <Input
                placeholder="YYYY-MM-DD"
                className="h-8 text-[12px]"
                value={filters.ctimeTo ?? ''}
                onChange={(e) => setFilters({ ctimeTo: e.target.value || undefined })}
              />
            </Field>
          </div>
          <div className="text-[11px] text-text-3">可写 YYYY-MM-DD 或 YYYY-MM-DDTHH:MM:SS</div>
        </div>
      </Capsule>

      <Capsule
        label="大小"
        active={sizes.some((v) => v !== undefined)}
        summary={
          filters.sizeMin !== undefined || filters.sizeMax !== undefined
            ? `${filters.sizeMin ?? 0}–${filters.sizeMax ?? '∞'} B`
            : null
        }
        onClear={() => setFilters({ sizeMin: undefined, sizeMax: undefined })}
      >
        <div className="grid grid-cols-2 gap-2">
          <Field label="最小（字节）">
            <Input
              type="number"
              min={0}
              className="h-8 text-[12px]"
              value={filters.sizeMin ?? ''}
              onChange={(e) => setFilters({ sizeMin: numOrUndef(e.target.value) })}
            />
          </Field>
          <Field label="最大（字节）">
            <Input
              type="number"
              min={0}
              className="h-8 text-[12px]"
              value={filters.sizeMax ?? ''}
              onChange={(e) => setFilters({ sizeMax: numOrUndef(e.target.value) })}
            />
          </Field>
        </div>
      </Capsule>

      <Capsule
        label="用户 / 组"
        active={who.some((v) => v !== undefined)}
        summary={who.filter((v) => v !== undefined).length + ' 项'}
        onClear={() => setFilters({ excludeUsers: undefined, excludeGroups: undefined })}
      >
        <div className="flex flex-col gap-3">
          <Field label="排除用户（逗号分隔）">
            <ListInput
              value={filters.excludeUsers}
              placeholder="root, daemon"
              onChange={(v) => setFilters({ excludeUsers: v })}
            />
          </Field>
          <Field label="排除组（逗号分隔）">
            <ListInput
              value={filters.excludeGroups}
              placeholder="wheel"
              onChange={(v) => setFilters({ excludeGroups: v })}
            />
          </Field>
        </div>
      </Capsule>

      <Capsule
        label="名称"
        active={filters.excludeNames !== undefined}
        summary={filters.excludeNames ? `${filters.excludeNames.length} 条` : null}
        onClear={() => setFilters({ excludeNames: undefined })}
      >
        <Field label="排除名称（每行一条正则）">
          <ListInput
            multiline
            value={filters.excludeNames}
            placeholder={'^node_modules$\n.*\\.tmp$'}
            onChange={(v) => setFilters({ excludeNames: v })}
          />
        </Field>
      </Capsule>

      <Capsule
        label="排除目录"
        active={filters.excludeDirs !== undefined}
        summary={filters.excludeDirs ? `${filters.excludeDirs.length} 条` : null}
        onClear={() => setFilters({ excludeDirs: undefined })}
      >
        <Field label="排除目录名（每行一条正则）">
          <ListInput
            multiline
            value={filters.excludeDirs}
            placeholder={'^\\.git$\n^node_modules$'}
            onChange={(v) => setFilters({ excludeDirs: v })}
          />
        </Field>
      </Capsule>
    </div>
  );
}
