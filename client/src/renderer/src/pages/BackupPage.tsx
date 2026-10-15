import type { ReactElement } from 'react';
import { FolderOpen } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
const FILTER_CAPSULES = ['类型', '时间', '大小', '用户/组', '名称', '排除目录'] as const;

/**
 * 备份页（ui-design.md §5.1）：居中的备份编排卡。
 * C0 只搭骨架（全部控件禁用），字段→CLI 转换与任务状态机由 C1 接入。
 */
export function BackupPage(): ReactElement {
  return (
    <div className="flex h-full items-center justify-center p-8">
      <Card className="w-full max-w-[880px] p-6" data-testid="backup-card">
        <div className="flex items-center gap-3">
          <Input
            placeholder="选择要备份的目录…"
            disabled
            className="h-11 flex-1 font-mono"
            aria-label="源目录"
          />
          <Button variant="secondary" disabled className="h-11">
            <FolderOpen className="size-4" />
            选择目录
          </Button>
        </div>

        <div className="mt-4 flex flex-wrap gap-2" aria-label="筛选条件">
          {FILTER_CAPSULES.map((c) => (
            <Button key={c} variant="outline" size="sm" disabled>
              {c}
            </Button>
          ))}
        </div>

        <div className="mt-4 flex items-center gap-6 text-sm text-text-2">
          <label className="flex items-center gap-2">
            <Switch disabled aria-label="压缩" />
            压缩
          </label>
          <label className="flex items-center gap-2">
            <Switch disabled aria-label="加密" />
            加密
          </label>
        </div>

        <div className="mt-4 flex items-center justify-between">
          <span className="text-sm text-text-3">—</span>
          <Button disabled className="px-8">
            开始备份
          </Button>
        </div>
      </Card>
    </div>
  );
}
