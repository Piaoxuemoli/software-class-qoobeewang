import type { ReactElement } from 'react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useAppStore } from '../stores/app';

/** 设置页（ui-design.md §5.3）：仓库路径、动效、关于。 */
export function SettingsPage(): ReactElement {
  const reduceMotion = useAppStore((s) => s.reduceMotion);
  const setReduceMotion = useAppStore((s) => s.setReduceMotion);
  const backendVersion = useAppStore((s) => s.backendVersion);
  const backendStatus = useAppStore((s) => s.backendStatus);

  return (
    <div className="mx-auto flex h-full w-full max-w-[880px] flex-col gap-4 overflow-auto p-6">
      <Card>
        <CardHeader>
          <CardTitle>备份仓库</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center gap-3">
          <Input
            value="~/DataBackup"
            disabled
            className="flex-1 font-mono"
            aria-label="备份仓库路径"
          />
          <Button variant="secondary" disabled>
            更改
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>动效</CardTitle>
        </CardHeader>
        <CardContent>
          <label className="flex items-center justify-between text-sm">
            <span>
              减少动效
              <span className="mt-0.5 block text-xs text-text-3">
                关闭过渡与滚动动画（系统的"减少动效"设置始终生效）
              </span>
            </span>
            <Switch
              checked={reduceMotion}
              onCheckedChange={setReduceMotion}
              aria-label="减少动效"
            />
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>关于</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-1.5 text-sm text-text-2">
          <div className="flex justify-between">
            <span>客户端</span>
            <span className="font-mono tnum">0.1.0</span>
          </div>
          <div className="flex justify-between">
            <span>后端</span>
            <span className="font-mono tnum">
              {backendStatus === 'ready' ? backendVersion : '不可用'}
            </span>
          </div>
          <Separator className="my-2" />
          <p className="text-xs text-text-3">DataBackup — 数据备份软件 · 本项目使用 AI 辅助开发</p>
        </CardContent>
      </Card>
    </div>
  );
}
