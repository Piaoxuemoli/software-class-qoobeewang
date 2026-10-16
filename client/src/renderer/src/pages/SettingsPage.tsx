import { useState } from 'react';
import { parseApiError } from '@/shared/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useAppStore } from '../stores/app';
import { useBackupsStore } from '../stores/backups';
import { errorText } from '../lib/errorText';

/** 设置页（ui-design.md §5.3）：仓库目录、动效偏好、关于。 */
export function SettingsPage(): React.JSX.Element {
  const { reduceMotion, setReduceMotion, backendStatus, backendVersion, clientVersion } =
    useAppStore();
  const { repoPath, setRepo } = useBackupsStore();
  const [draft, setDraft] = useState<string | null>(null);
  const [msg, setMsg] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);
  const [saving, setSaving] = useState(false);

  const value = draft ?? repoPath;

  const pick = async (): Promise<void> => {
    const dir = await window.databackup.dialog.pickDirectory({ title: '选择备份仓库目录' });
    if (dir) setDraft(dir);
  };

  const save = async (): Promise<void> => {
    setSaving(true);
    setMsg(null);
    try {
      await setRepo(value);
      setDraft(null);
      setMsg({ kind: 'ok', text: '已保存' });
    } catch (err) {
      const e = parseApiError(err);
      setMsg({ kind: 'err', text: errorText(e.code, e.message).title });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex max-w-2xl flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-[15px]">备份仓库</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex items-end gap-2">
            <div className="flex-1">
              <Label htmlFor="repo" className="mb-1 text-[12px] text-text-2">
                仓库目录
              </Label>
              <Input
                id="repo"
                className="font-mono text-[13px]"
                value={value}
                onChange={(e) => setDraft(e.target.value)}
              />
            </div>
            <Button variant="secondary" onClick={() => void pick()}>
              更改…
            </Button>
            <Button
              onClick={() => void save()}
              disabled={saving || draft === null || draft === repoPath}
            >
              保存
            </Button>
          </div>
          {msg && (
            <p
              className={msg.kind === 'ok' ? 'text-[12px] text-success' : 'text-[12px] text-danger'}
            >
              {msg.text}
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-[15px]">动效</CardTitle>
        </CardHeader>
        <CardContent>
          <label className="flex items-center justify-between">
            <div>
              <div className="text-[14px]">减少动效</div>
              <div className="text-[12px] text-text-3">
                降低数字滚动与状态图标的动画幅度，同时遵循系统 prefers-reduced-motion
              </div>
            </div>
            <Switch
              aria-label="减少动效"
              checked={reduceMotion}
              onCheckedChange={setReduceMotion}
            />
          </label>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-[15px]">关于</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-1.5 text-[13px]">
          <div className="flex justify-between">
            <span className="text-text-3">客户端</span>
            <span className="font-mono">{clientVersion ?? '—'}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-3">后端</span>
            <span className="font-mono" data-testid="backend-version">
              {backendStatus === 'ready' ? backendVersion : '不可用'}
            </span>
          </div>
          <p className="mt-2 text-[12px] text-text-3">
            UESTC《软件开发综合实验》课程项目 · 组长：王科瑾
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
