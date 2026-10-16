import { useEffect } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useJobStore } from '../stores/job';
import { useAppStore } from '../stores/app';
import { useBackupsStore } from '../stores/backups';
import { FilterCapsules } from '../components/FilterCapsules';
import { RunningCard, SucceededCard, FailedCard, CancelledCard } from '../components/JobCard';
import { formatBytes, formatCount } from '../lib/format';
import { errorText } from '../lib/errorText';

/**
 * 备份页（ui-design.md §5.1）：
 * 编排卡（源目录 + 筛选胶囊 + 选项 + 开始）→ 进度卡 → 完成/错误/取消卡。
 */
export function BackupPage(): React.JSX.Element {
  const job = useJobStore();
  const setPage = useAppStore((s) => s.setPage);
  const refreshBackups = useBackupsStore((s) => s.refresh);
  const { form, status, preview } = job;

  // 表单变化后防抖 400ms 发 dry-run 预览（interaction.md §5.1：超过 2s 显示"计算中…"）。
  useEffect(() => {
    if (status !== 'editing' && status !== 'ready') return;
    const t = window.setTimeout(() => void job.refreshPreview(), 400);
    return () => window.clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form]);

  // 备份成功后列表缓存失效，刷新备份管理页数据。
  useEffect(() => {
    if (status === 'succeeded' && job.kind === 'backup') void refreshBackups();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status]);

  const pickSource = async (): Promise<void> => {
    const dir = await window.databackup.dialog.pickDirectory({ title: '选择要备份的目录' });
    if (dir) job.setSource(dir);
  };

  if (status === 'running' || status === 'cancelling') return <RunningCard />;
  if (status === 'succeeded') {
    return (
      <SucceededCard
        actions={
          <>
            <Button variant="secondary" onClick={() => setPage('manage')}>
              查看备份
            </Button>
            <Button onClick={job.again}>再次备份</Button>
          </>
        }
      />
    );
  }
  if (status === 'failed') {
    return (
      <FailedCard
        actions={
          <>
            <Button variant="ghost" onClick={job.retry}>
              返回编辑
            </Button>
            <Button onClick={job.retry}>重试</Button>
          </>
        }
      />
    );
  }
  if (status === 'cancelled') {
    return <CancelledCard actions={<Button onClick={job.backToReady}>返回</Button>} />;
  }

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 p-6">
        <div className="flex items-end gap-2">
          <div className="flex-1">
            <Label htmlFor="src" className="mb-1 text-[12px] text-text-2">
              源目录
            </Label>
            <Input
              id="src"
              placeholder="/path/to/source"
              className="font-mono text-[13px]"
              value={form.source}
              onChange={(e) => job.setSource(e.target.value)}
            />
          </div>
          <Button variant="secondary" onClick={() => void pickSource()}>
            浏览…
          </Button>
        </div>

        <div>
          <div className="mb-2 text-[12px] text-text-2">筛选条件</div>
          <FilterCapsules />
        </div>

        <div className="min-h-5 text-[12px] text-text-3" data-testid="dryrun-preview">
          {preview.pending
            ? '正在计算预览…'
            : preview.error
              ? `预览失败：${errorText(preview.error.code, preview.error.message).title}`
              : preview.summary
                ? `预计 ${formatCount(preview.summary.files)} 个文件 · ${formatCount(preview.summary.dirs)} 个目录 · ${formatBytes(preview.summary.bytes)}${preview.summary.skipped > 0 ? ` · 跳过 ${preview.summary.skipped}` : ''}`
                : form.source.trim() === ''
                  ? ''
                  : '筛选条件变化时将自动预览'}
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <Label htmlFor="bname" className="mb-1 text-[12px] text-text-2">
              备份名称（可选）
            </Label>
            <Input
              id="bname"
              placeholder="默认：目录名-时间戳"
              className="text-[13px]"
              value={form.name}
              onChange={(e) => job.setName(e.target.value)}
            />
          </div>
        </div>

        <div className="flex items-center gap-6">
          <label className="flex items-center gap-2 text-[13px]">
            <Switch checked={form.compress} onCheckedChange={job.setCompress} />
            压缩
          </label>
          <label className="flex items-center gap-2 text-[13px]">
            <Switch checked={form.encrypt} onCheckedChange={job.setEncrypt} />
            加密
          </label>
        </div>

        {form.encrypt && (
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="pw" className="mb-1 text-[12px] text-text-2">
                口令
              </Label>
              <Input
                id="pw"
                type="password"
                className="text-[13px]"
                value={form.password}
                onChange={(e) => job.setPassword(e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="pw2" className="mb-1 text-[12px] text-text-2">
                确认口令
              </Label>
              <Input
                id="pw2"
                type="password"
                className="text-[13px]"
                value={form.confirm}
                onChange={(e) => job.setConfirm(e.target.value)}
              />
            </div>
            {form.password !== '' && form.confirm !== '' && form.password !== form.confirm && (
              <p className="col-span-2 text-[12px] text-warning">两次输入的口令不一致</p>
            )}
          </div>
        )}

        <div className="flex justify-end">
          <Button onClick={() => void job.start()} disabled={status !== 'ready'}>
            开始备份
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
