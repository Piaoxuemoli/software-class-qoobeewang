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

/** 侧栏（ui-design.md §4）：220px，导航 + 仓库路径 + 后端状态；可折叠为 64px 图标栏。 */
export function Sidebar(): React.JSX.Element {
  const page = useAppStore((s) => s.page);
  const setPage = useAppStore((s) => s.setPage);
  const collapsed = useAppStore((s) => s.sidebarCollapsed);
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
    <aside
      data-testid="sidebar"
      data-collapsed={collapsed}
      className={cn(
        'flex shrink-0 flex-col border-r border-border bg-surface-1',
        collapsed ? 'w-16' : 'w-[220px]',
      )}
    >
      <nav className="flex flex-col gap-0.5 p-3" aria-label="主导航">
        {NAV.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => setPage(id)}
            aria-current={page === id ? 'page' : undefined}
            aria-label={label}
            title={collapsed ? label : undefined}
            className={cn(
              'flex items-center gap-2.5 rounded-full py-2 text-sm transition-colors',
              collapsed ? 'justify-center px-0' : 'px-3.5',
              page === id
                ? 'bg-surface-2 font-medium text-text'
                : 'text-text-2 hover:bg-surface-2 hover:text-text',
            )}
          >
            <Icon className="size-4 shrink-0" />
            {!collapsed && label}
          </button>
        ))}
      </nav>
      <div className="mt-auto flex flex-col gap-2 p-3">
        <Separator />
        {!collapsed && (
          <div className="px-1 font-mono text-xs text-text-3" title="备份仓库路径">
            ~/DataBackup
          </div>
        )}
        <div
          data-testid="statusbar"
          className={cn(
            'flex items-center gap-2 px-1 text-xs text-text-2',
            collapsed && 'justify-center',
          )}
          title={statusText}
        >
          <StatusDot status={backendStatus} />
          <span className={collapsed ? 'sr-only' : 'truncate'}>{statusText}</span>
        </div>
      </div>
    </aside>
  );
}
