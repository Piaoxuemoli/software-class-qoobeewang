import { useEffect, useState } from 'react';

type TabId = 'backup' | 'manage' | 'settings';

const TABS: { id: TabId; label: string }[] = [
  { id: 'backup', label: '备份' },
  { id: 'manage', label: '管理备份数据' },
  { id: 'settings', label: '备份设置' },
];

const TAB_HINT: Record<TabId, string> = {
  backup: '选择目录并创建备份（待实现）',
  manage: '查看、还原与删除已有备份（待实现）',
  settings: '过滤规则、压缩与加密等备份选项（待实现）',
};

export default function App() {
  const [tab, setTab] = useState<TabId>('backup');
  const [backendStatus, setBackendStatus] = useState('正在连接后端…');

  useEffect(() => {
    let cancelled = false;
    window.databackup
      .getBackendVersion()
      .then((v) => {
        if (!cancelled) setBackendStatus(`后端版本: ${v}`);
      })
      .catch((err: unknown) => {
        if (!cancelled)
          setBackendStatus(`后端不可用: ${err instanceof Error ? err.message : String(err)}`);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="app">
      <header className="titlebar">Data Backup</header>
      <nav className="tabs">
        {TABS.map((t) => (
          <button
            key={t.id}
            className={tab === t.id ? 'tab active' : 'tab'}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </nav>
      <main className="page">
        <h2>{TABS.find((t) => t.id === tab)?.label}</h2>
        <p>{TAB_HINT[tab]}</p>
      </main>
      <footer className="statusbar" data-testid="statusbar">
        {backendStatus}
      </footer>
    </div>
  );
}
