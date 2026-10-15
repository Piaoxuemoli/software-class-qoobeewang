import { useEffect, type ReactElement } from 'react';

import { Toaster } from '@/components/ui/sonner';
import { TitleBar } from './components/TitleBar';
import { Sidebar } from './components/Sidebar';
import { BackupPage } from './pages/BackupPage';
import { ManagePage } from './pages/ManagePage';
import { SettingsPage } from './pages/SettingsPage';
import { useAppStore, type PageId } from './stores/app';

const PAGES: Record<PageId, () => ReactElement> = {
  backup: BackupPage,
  manage: ManagePage,
  settings: SettingsPage,
};

export default function App(): ReactElement {
  const page = useAppStore((s) => s.page);
  const reduceMotion = useAppStore((s) => s.reduceMotion);
  const setBackendReady = useAppStore((s) => s.setBackendReady);
  const setBackendUnavailable = useAppStore((s) => s.setBackendUnavailable);

  // 启动时探测后端版本（interaction.md §4.1）。
  useEffect(() => {
    let cancelled = false;
    window.databackup
      .getBackendVersion()
      .then((v) => {
        if (!cancelled) setBackendReady(v);
      })
      .catch((err: unknown) => {
        if (!cancelled) setBackendUnavailable(err instanceof Error ? err.message : String(err));
      });
    return () => {
      cancelled = true;
    };
  }, [setBackendReady, setBackendUnavailable]);

  // 应用内"减少动效"开关（叠加在 prefers-reduced-motion 之上）。
  useEffect(() => {
    document.documentElement.dataset.motion = reduceMotion ? 'reduce' : 'standard';
  }, [reduceMotion]);

  const Page = PAGES[page];
  return (
    <div className="flex h-full flex-col">
      <TitleBar />
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <main className="min-w-0 flex-1 overflow-hidden">
          {/* 页面切换 150ms 淡入（interaction.md §5）；reduce-motion 下时长归零 */}
          <div key={page} className="h-full animate-in fade-in-0 duration-150">
            <Page />
          </div>
        </main>
      </div>
      <Toaster position="bottom-right" />
    </div>
  );
}
