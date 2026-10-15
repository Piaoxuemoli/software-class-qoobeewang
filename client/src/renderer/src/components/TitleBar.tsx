import { useEffect, useState, type ReactElement, type ReactNode } from 'react';
import { Copy, Minus, Square, X } from 'lucide-react';

import { cn } from '@/lib/utils';
import { useAppStore, type PageId } from '../stores/app';

export const PAGE_TITLE: Record<PageId, string> = {
  backup: '备份',
  manage: '备份管理',
  settings: '设置',
};

function ControlButton({
  label,
  danger = false,
  onClick,
  children,
}: {
  label: string;
  danger?: boolean;
  onClick: () => void;
  children: ReactNode;
}): ReactElement {
  return (
    <button
      type="button"
      aria-label={label}
      onClick={onClick}
      className={cn(
        'app-no-drag flex h-10 w-[46px] items-center justify-center text-text-2 transition-colors',
        danger ? 'hover:bg-danger hover:text-white' : 'hover:bg-surface-2 hover:text-text',
      )}
    >
      {children}
    </button>
  );
}

/** 自绘标题栏（ui-design.md §3）：40px，整条可拖拽，双击切换最大化。 */
export function TitleBar(): ReactElement {
  const page = useAppStore((s) => s.page);
  const [maximized, setMaximized] = useState(false);

  useEffect(() => window.databackup.window.onMaximizedChange(setMaximized), []);

  const win = window.databackup.window;
  return (
    <header
      className="app-drag flex h-10 shrink-0 select-none items-center border-b border-border"
      onDoubleClick={() => win.toggleMaximize()}
    >
      <div className="w-[220px] shrink-0 px-4 text-sm font-medium text-text-2">DataBackup</div>
      <div className="flex-1 text-center text-[13px] text-text-3">{PAGE_TITLE[page]}</div>
      <div className="flex shrink-0" onDoubleClick={(e) => e.stopPropagation()}>
        <ControlButton label="最小化" onClick={() => win.minimize()}>
          <Minus className="size-4" />
        </ControlButton>
        <ControlButton label={maximized ? '还原' : '最大化'} onClick={() => win.toggleMaximize()}>
          {maximized ? <Copy className="size-3.5" /> : <Square className="size-3.5" />}
        </ControlButton>
        <ControlButton label="关闭" danger onClick={() => win.close()}>
          <X className="size-4" />
        </ControlButton>
      </div>
    </header>
  );
}
