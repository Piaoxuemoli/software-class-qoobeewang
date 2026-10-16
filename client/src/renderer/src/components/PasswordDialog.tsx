import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useJobStore } from '../stores/job';

/**
 * 加密备份的口令对话框（interaction.md §5 弹窗）。
 * 打开/关闭完全由 jobStore.status === 'awaitingPassword' 驱动；
 * E_AUTH 回到本态并在下方提示剩余次数，连续 3 次失败由 store 送回 ready。
 */
export function PasswordDialog(): React.JSX.Element {
  const { status, restore, authFailures, error, setRestorePassword, submitRestorePassword } =
    useJobStore();
  const open = status === 'awaitingPassword' && restore !== null;

  const giveUp = (): void => {
    useJobStore.setState({ status: 'ready', error: null });
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && giveUp()}>
      <DialogContent className="sm:max-w-md" data-testid="password-dialog">
        <DialogHeader>
          <DialogTitle>输入备份口令</DialogTitle>
        </DialogHeader>
        <p className="text-[13px] text-text-3">
          「{restore?.name}」已加密，需要口令才能还原。口令只传给后端，不保存在界面。
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submitRestorePassword();
          }}
        >
          <Input
            type="password"
            autoFocus
            aria-label="备份口令"
            placeholder="口令"
            value={restore?.password ?? ''}
            onChange={(e) => setRestorePassword(e.target.value)}
          />
          {error && authFailures > 0 && (
            <p className="mt-2 text-[12px] text-danger" role="alert">
              口令错误或备份被篡改，还可尝试 {3 - authFailures} 次
            </p>
          )}
          {error && error.code === 'E_PASSWORD_REQUIRED' && (
            <p className="mt-2 text-[12px] text-warning" role="alert">
              该备份已加密，需要口令
            </p>
          )}
          <div className="mt-4 flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={giveUp}>
              取消
            </Button>
            <Button type="submit" disabled={(restore?.password ?? '') === ''}>
              开始还原
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
