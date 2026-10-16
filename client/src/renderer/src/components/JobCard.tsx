import { useEffect, useState } from 'react';
import type { ProgressEvent, RestoreResult, BackupResult } from '@/shared/protocol';
import { Progress } from '@/components/ui/progress';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useJobStore } from '../stores/job';
import { Counter } from './Counter';
import { StatusMark } from './StatusMark';
import { formatBytes, formatCount, formatElapsed, formatMs } from '../lib/format';
import { errorText } from '../lib/errorText';

const PHASE_TEXT: Record<string, string> = {
  scan: '扫描中',
  pack: '处理中',
  restore: '还原中',
};

function useElapsed(startedAt: number | null, active: boolean): number {
  const [sec, setSec] = useState(0);
  useEffect(() => {
    if (!active || startedAt === null) return;
    setSec(0);
    const t = window.setInterval(() => setSec(Math.floor((Date.now() - startedAt) / 1000)), 500);
    return () => window.clearInterval(t);
  }, [startedAt, active]);
  return startedAt === null ? 0 : sec;
}

/** 进度的百分比；总量未知（扫描阶段）时返回 null 显示不确定条。 */
function percentOf(p: ProgressEvent | null): number | null {
  const tf = p?.totalFiles ?? 0;
  const tb = p?.totalBytes ?? 0;
  if (!p) return null;
  if (tf > 0) return Math.min(100, Math.round((p.files / tf) * 100));
  if (tb > 0) return Math.min(100, Math.round((p.bytes / tb) * 100));
  return null;
}

function Stat({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <div>
      <div className="text-[11px] text-text-3">{label}</div>
      <div className="font-mono text-[15px] text-text-1" data-testid={`stat-${label}`}>
        {children}
      </div>
    </div>
  );
}

/** running / cancelling 态的进度卡（备份页与还原 Sheet 共用）。 */
export function RunningCard(): React.JSX.Element {
  const { status, kind, phase, progress, logs, startedAt, cancel } = useJobStore();
  const elapsed = useElapsed(startedAt, status === 'running');
  const pct = percentOf(progress);
  const title =
    status === 'cancelling'
      ? '正在取消…'
      : `${kind === 'restore' ? '还原中' : '备份中'}${phase ? ` · ${PHASE_TEXT[phase] ?? phase}` : ''}`;

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 p-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <StatusMark kind="running" />
            <span className="text-[15px] font-medium">{title}</span>
          </div>
          <span className="font-mono text-[13px] text-text-3" data-testid="elapsed">
            {formatElapsed(elapsed)}
          </span>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <Stat label="文件">
            <Counter value={progress?.files ?? 0} format={(n) => formatCount(Math.floor(n))} />
            {progress?.totalFiles !== undefined && progress.totalFiles > 0 && (
              <span className="text-text-3"> / {formatCount(progress.totalFiles)}</span>
            )}
          </Stat>
          <Stat label="大小">
            <Counter value={progress?.bytes ?? 0} format={formatBytes} />
            {progress?.totalBytes !== undefined && progress.totalBytes > 0 && (
              <span className="text-text-3"> / {formatBytes(progress.totalBytes)}</span>
            )}
          </Stat>
        </div>

        {pct === null ? (
          <div className="relative h-1 w-full overflow-hidden rounded-full bg-surface-3">
            <div className="indeterminate-bar" />
          </div>
        ) : (
          <div className="flex items-center gap-3">
            <Progress value={pct} className="flex-1" />
            <span
              className="w-10 text-right font-mono text-[12px] text-text-2"
              data-testid="job-percent"
            >
              {pct}%
            </span>
          </div>
        )}

        {progress?.current && (
          <div className="truncate font-mono text-[12px] text-text-3" data-testid="current-path">
            {progress.current}
          </div>
        )}

        {logs.length > 0 && (
          <details className="rounded-[var(--radius-md)] border border-border bg-surface-2 px-3 py-2">
            <summary className="cursor-pointer text-[12px] text-text-2">
              日志（{logs.length} 条）
            </summary>
            <div className="mt-2 max-h-40 overflow-y-auto font-mono text-[11px] leading-5 text-text-3">
              {logs.map((l, i) => (
                <div
                  key={i}
                  className={l.level === 'warn' || l.level === 'error' ? 'text-warning' : undefined}
                >
                  [{l.level}] {l.message}
                </div>
              ))}
            </div>
          </details>
        )}

        <div className="flex justify-end">
          <Button variant="ghost" onClick={cancel} disabled={status === 'cancelling'}>
            取消
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function isBackup(r: BackupResult | RestoreResult): r is BackupResult {
  return r.command === 'backup';
}

/** succeeded 态的完成卡；actions 由页面注入（查看备份 / 再次备份等）。 */
export function SucceededCard({ actions }: { actions?: React.ReactNode }): React.JSX.Element {
  const { result } = useJobStore();
  if (!result || result.type !== 'result') return <></>;
  const ratio =
    isBackup(result) && result.bytes > 0
      ? `${((result.outputBytes / result.bytes) * 100).toFixed(1)}%`
      : null;
  return (
    <Card>
      <CardContent className="flex flex-col gap-4 p-6">
        <div className="flex items-center gap-3">
          <StatusMark kind="success" />
          <span className="text-[15px] font-medium" data-testid="job-succeeded">
            {isBackup(result) ? '备份完成' : '还原完成'}
          </span>
        </div>
        <div className="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-4">
          <Stat label="文件">{formatCount(result.files)}</Stat>
          <Stat label="目录">{formatCount(result.dirs)}</Stat>
          <Stat label="数据量">{formatBytes(result.bytes)}</Stat>
          <Stat label="耗时">{formatMs(result.ms)}</Stat>
          {isBackup(result) && <Stat label="输出文件">{formatBytes(result.outputBytes)}</Stat>}
          {ratio && <Stat label="压缩率">{ratio}</Stat>}
        </div>
        <div className="truncate font-mono text-[12px] text-text-3">
          {isBackup(result) ? result.path : result.target}
        </div>
        {actions && <div className="flex justify-end gap-2">{actions}</div>}
      </CardContent>
    </Card>
  );
}

/** failed 态的错误卡。 */
export function FailedCard({ actions }: { actions?: React.ReactNode }): React.JSX.Element {
  const { error } = useJobStore();
  const t = error ? errorText(error.code, error.message) : { title: '任务失败', hint: '' };
  return (
    <Card className="border-danger/40">
      <CardContent className="flex flex-col gap-3 p-6">
        <div className="flex items-center gap-3">
          <StatusMark kind="error" />
          <span className="text-[15px] font-medium text-danger" data-testid="job-failed">
            {t.title}
          </span>
        </div>
        {t.hint && <div className="pl-8 text-[13px] text-text-3">{t.hint}</div>}
        {actions && <div className="flex justify-end gap-2">{actions}</div>}
      </CardContent>
    </Card>
  );
}

/** cancelled 态。 */
export function CancelledCard({ actions }: { actions?: React.ReactNode }): React.JSX.Element {
  return (
    <Card>
      <CardContent className="flex flex-col gap-3 p-6">
        <div className="flex items-center gap-3">
          <StatusMark kind="warning" />
          <span className="text-[15px] font-medium" data-testid="job-cancelled">
            已取消
          </span>
        </div>
        {actions && <div className="flex justify-end gap-2">{actions}</div>}
      </CardContent>
    </Card>
  );
}
