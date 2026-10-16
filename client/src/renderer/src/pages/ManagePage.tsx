import { useState } from 'react';
import { parseApiError, type ApiError, type BackupInfo } from '@/shared/api';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { useBackupsStore } from '../stores/backups';
import { useJobStore } from '../stores/job';
import { useAppStore } from '../stores/app';
import { HoldButton } from '../components/HoldButton';
import { PasswordDialog } from '../components/PasswordDialog';
import { RunningCard, SucceededCard, FailedCard, CancelledCard } from '../components/JobCard';
import { formatBytes, formatCount, formatTime } from '../lib/format';
import { errorText } from '../lib/errorText';

function DetailRow({ label, value }: { label: string; value: string }): React.JSX.Element {
  return (
    <div className="flex justify-between gap-4 text-[13px]">
      <span className="text-text-3">{label}</span>
      <span className="truncate font-mono text-text-1">{value}</span>
    </div>
  );
}

/** 详情 Sheet 内的还原区：目标目录 → （加密）口令 → 进度 → 结果。 */
function RestorePanel({ backup }: { backup: BackupInfo }): React.JSX.Element {
  const job = useJobStore();
  const r = job.restore;

  if (job.status === 'running' || job.status === 'cancelling') return <RunningCard />;
  if (job.status === 'succeeded') {
    return <SucceededCard actions={<Button onClick={job.endRestore}>关闭</Button>} />;
  }
  if (job.status === 'failed') {
    return (
      <FailedCard
        actions={
          <>
            <Button variant="ghost" onClick={job.endRestore}>
              关闭
            </Button>
            <Button onClick={job.retry}>重试</Button>
          </>
        }
      />
    );
  }
  if (job.status === 'cancelled') {
    return <CancelledCard actions={<Button onClick={job.backToReady}>返回</Button>} />;
  }

  const pickTarget = async (): Promise<void> => {
    const dir = await window.databackup.dialog.pickDirectory({ title: '选择还原目标目录' });
    if (dir) job.setRestoreTarget(dir);
  };

  return (
    <div className="flex flex-col gap-3" data-testid="restore-panel">
      <div className="text-[13px] font-medium">还原到</div>
      <div className="flex items-end gap-2">
        <div className="flex-1">
          <Label htmlFor="target" className="mb-1 text-[12px] text-text-2">
            目标目录（须为空或不存在）
          </Label>
          <Input
            id="target"
            placeholder="/path/to/target"
            className="font-mono text-[13px]"
            value={r?.target ?? ''}
            onChange={(e) => job.setRestoreTarget(e.target.value)}
          />
        </div>
        <Button variant="secondary" onClick={() => void pickTarget()}>
          浏览…
        </Button>
      </div>
      {backup.encrypted && (
        <p className="text-[12px] text-text-3">该备份已加密，开始后需要输入口令。</p>
      )}
      <div className="flex justify-end gap-2">
        <Button variant="ghost" onClick={job.endRestore}>
          取消
        </Button>
        <Button onClick={() => void job.requestRestore()} disabled={job.status !== 'ready'}>
          开始还原
        </Button>
      </div>
    </div>
  );
}

/** 备份详情 Sheet：元信息 + 还原 + 长按删除。 */
function DetailSheet({ backup }: { backup: BackupInfo }): React.JSX.Element {
  const { select, remove } = useBackupsStore();
  const job = useJobStore();
  const [delErr, setDelErr] = useState<ApiError | null>(null);
  const inRestore = job.kind === 'restore' && job.restore?.name === backup.name;

  const doDelete = async (): Promise<void> => {
    try {
      await remove(backup.name);
      select(null);
    } catch (err) {
      setDelErr(parseApiError(err));
    }
  };

  return (
    <Sheet
      open
      onOpenChange={(o) => {
        if (!o) {
          select(null);
          if (inRestore && job.status !== 'running' && job.status !== 'cancelling') {
            job.endRestore();
          }
        }
      }}
    >
      <SheetContent className="flex flex-col gap-5 overflow-y-auto" data-testid="backup-sheet">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2 font-mono">
            {backup.name}
            {backup.encrypted && <Badge variant="secondary">加密</Badge>}
            {backup.corrupt && <Badge variant="danger">已损坏</Badge>}
          </SheetTitle>
          <SheetDescription className="sr-only">备份详情</SheetDescription>
        </SheetHeader>

        {backup.corrupt ? (
          <p className="text-[13px] text-text-3">该备份文件已损坏，无法读取元信息，只能删除。</p>
        ) : (
          <div className="flex flex-col gap-1.5">
            <DetailRow label="创建时间" value={formatTime(backup.createdAt)} />
            <DetailRow label="文件数" value={formatCount(backup.files)} />
            <DetailRow label="目录数" value={formatCount(backup.dirs)} />
            <DetailRow label="数据量" value={formatBytes(backup.bytes)} />
            <DetailRow label="输出大小" value={formatBytes(backup.outputBytes)} />
            <DetailRow label="压缩" value={backup.compressed ? 'Huffman' : '无'} />
            <DetailRow label="加密" value={backup.encrypted ? 'AES-256-CBC' : '无'} />
            <DetailRow label="源目录" value={backup.source} />
            <DetailRow label="格式版本" value={String(backup.formatVersion)} />
          </div>
        )}

        {inRestore ? (
          <RestorePanel backup={backup} />
        ) : (
          <div className="mt-auto flex items-center justify-between">
            <HoldButton onConfirm={() => void doDelete()} disabled={job.status === 'running'}>
              删除此备份
            </HoldButton>
            {!backup.corrupt && <Button onClick={() => job.beginRestore(backup)}>还原…</Button>}
          </div>
        )}

        {delErr && (
          <p className="text-[12px] text-danger" role="alert">
            {errorText(delErr.code, delErr.message).title}
          </p>
        )}
      </SheetContent>
    </Sheet>
  );
}

export function ManagePage(): React.JSX.Element {
  const { backups, loading, error, repoPath, refresh, selected, select } = useBackupsStore();
  const [query, setQuery] = useState('');
  const setPage = useAppStore((s) => s.setPage);

  const list = (backups ?? []).filter((b) => b.name.toLowerCase().includes(query.toLowerCase()));
  const selBackup = backups?.find((b) => b.name === selected) ?? null;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <Input
          placeholder="搜索备份名…"
          className="w-64 text-[13px]"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label="搜索备份"
        />
        <Button variant="secondary" onClick={() => void refresh()}>
          刷新
        </Button>
        <button
          type="button"
          className="ml-auto truncate font-mono text-[12px] text-text-3 hover:text-text-1"
          title="点击前往设置修改仓库目录"
          onClick={() => setPage('settings')}
        >
          {repoPath}
        </button>
      </div>

      {loading && backups === null ? (
        <div className="flex flex-col gap-2" data-testid="list-skeleton">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      ) : error && backups === null ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-2 p-8 text-center">
            <p className="text-[14px] text-danger">{errorText(error.code, error.message).title}</p>
            <Button variant="secondary" onClick={() => void refresh()}>
              重试
            </Button>
          </CardContent>
        </Card>
      ) : list.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center">
            <p className="text-[14px] text-text-2" data-testid="empty-list">
              {query ? '没有匹配的备份' : '还没有备份'}
            </p>
            {!query && (
              <Button className="mt-4" onClick={() => setPage('backup')}>
                去备份
              </Button>
            )}
          </CardContent>
        </Card>
      ) : (
        <div className="overflow-hidden rounded-[var(--radius-md)] border border-border">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-border bg-surface-1 text-left text-[12px] text-text-3">
                <th className="px-4 py-2 font-medium">名称</th>
                <th className="px-4 py-2 font-medium">创建时间</th>
                <th className="px-4 py-2 font-medium">大小</th>
                <th className="px-4 py-2 font-medium">加密</th>
                <th className="px-4 py-2 font-medium">完整性</th>
              </tr>
            </thead>
            <tbody>
              {list.map((b) => (
                <tr
                  key={b.name}
                  className="cursor-pointer border-b border-border/50 bg-surface-0 transition-colors last:border-0 hover:bg-surface-2"
                  onClick={() => select(b.name)}
                  data-testid={`backup-row-${b.name}`}
                >
                  <td className="px-4 py-2.5 font-mono">{b.name}</td>
                  <td className="px-4 py-2.5 text-text-2">{formatTime(b.createdAt)}</td>
                  <td className="px-4 py-2.5 font-mono text-text-2">
                    {b.corrupt ? '—' : formatBytes(b.outputBytes)}
                  </td>
                  <td className="px-4 py-2.5 text-text-2">
                    {b.corrupt ? '—' : b.encrypted ? '是' : '否'}
                  </td>
                  <td className="px-4 py-2.5">
                    {b.corrupt ? (
                      <span className="text-danger">已损坏</span>
                    ) : (
                      <span className="text-success">正常</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selBackup && <DetailSheet backup={selBackup} />}
      <PasswordDialog />
    </div>
  );
}
