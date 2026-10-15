import { Archive, DatabaseBackup, Settings } from 'lucide-react';

import { Separator } from '@/components/ui/separator';
import { cn } from '@/lib/utils';
import { useAppStore, type PageId } from '../stores/app';

const NAV: { id: PageId; label: string; icon: typeof DatabaseBackup }[] = [
  { id: 'backup', label: '备份', icon: DatabaseBackup },
  { id: 'manage', label: '备份管理', icon: Archive },
  { id: 'settings', label: '设置', icon: Settings },
];

function StatusDot({
  status,
}: {
  status: 'connecting' | 'ready' | 'unavailable';
}): React.JSX.Element {
  return (
    <span
      className={cn(
        'inline-block size-1.5 shrink-0 rounded-full',
        status === 'ready' && 'bg-success',
        status === 'unavailable' && 'bg-danger',
        status === 'connecting' && 'bg-text-3',
      )}
    />
  );
}

/** 侧栏（ui-design.md §4）：220px，导航 + 仓库路径 + 后端状态。 */
export function Sidebar(): React.JSX.Element {
  const page = useAppStore((s) => s.page);
  const setPage = useAppStore((s) => s.setPage);
  const backendStatus = useAppStore((s) => s.backendStatus);
  const backendVersion = useAppStore((s) => s.backendVersion);
  const backendError = useAppStore((s) => s.backendError);

  const statusText =
    backendStatus === 'ready'
      ? `后端版本: ${backendVersion}`
      : backendStatus === 'unavailable'
        ? `后端不可用: ${backendError ?? '未知错误'}`
        : '正在连接后端…';

  return (
    <aside className="flex w-[220px] shrink-0 flex-col border-r border-border bg-surface-1">
      <nav className="flex flex-col gap-0.5 p-3" aria-label="主导航">
        {NAV.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => setPage(id)}
            aria-current={page === id ? 'page' : undefined}
            className={cn(
              'flex items-center gap-2.5 rounded-full px-3.5 py-2 text-sm transition-colors',
              page === id
                ? 'bg-surface-2 font-medium text-text'
                : 'text-text-2 hover:bg-surface-2 hover:text-text',
            )}
          >
            <Icon className="size-4" />
            {label}
          </button>
        ))}
      </nav>
      <div className="mt-auto flex flex-col gap-2 p-3">
        <Separator />
        <div className="px-1 font-mono text-xs text-text-3" title="备份仓库路径">
          ~/DataBackup
        </div>
        <div
          data-testid="statusbar"
          className="flex items-center gap-2 px-1 text-xs text-text-2"
          title={statusText}
        >
          <StatusDot status={backendStatus} />
          <span className="truncate">{statusText}</span>
        </div>
      </div>
    </aside>
  );
}
