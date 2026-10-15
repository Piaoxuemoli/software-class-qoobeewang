import type { ReactElement } from 'react';
import { Archive, RefreshCw, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAppStore } from '../stores/app';

/** 备份管理页（ui-design.md §5.2）。C0 为骨架：搜索/刷新禁用 + 空状态引导。 */
export function ManagePage(): ReactElement {
  const setPage = useAppStore((s) => s.setPage);

  return (
    <div className="flex h-full flex-col p-6">
      <div className="flex items-center gap-3">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-text-3" />
          <Input placeholder="搜索备份…" disabled className="pl-9" aria-label="搜索备份" />
        </div>
        <Button variant="secondary" size="icon" disabled aria-label="刷新">
          <RefreshCw className="size-4" />
        </Button>
      </div>
      <div className="mt-2 font-mono text-xs text-text-3">~/DataBackup</div>

      <div className="flex flex-1 flex-col items-center justify-center gap-4 text-center">
        <Archive className="size-10 text-text-3" />
        <div>
          <p className="text-sm text-text-2">暂无备份</p>
          <p className="mt-1 text-xs text-text-3">创建第一个备份后会显示在这里</p>
        </div>
        <Button variant="secondary" onClick={() => setPage('backup')}>
          去备份
        </Button>
      </div>
    </div>
  );
}
