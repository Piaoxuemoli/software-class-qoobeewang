import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useBackupsStore, initialBackupsState } from './backups';
import { installFakeApi, fakeBackups, type FakeApi } from '../test/fakeApi';

let api: FakeApi;
const store = (): ReturnType<typeof useBackupsStore.getState> => useBackupsStore.getState();

beforeEach(() => {
  api = installFakeApi();
  useBackupsStore.setState({ ...initialBackupsState });
});

describe('backupsStore（interaction.md §1）', () => {
  it('TC-UI-021 refresh 读取仓库路径与列表', async () => {
    await store().refresh();
    expect(api.config.get).toHaveBeenCalled();
    expect(api.repo.list).toHaveBeenCalled();
    expect(store().repoPath).toBe('/repo');
    expect(store().backups).toHaveLength(2);
    expect(store().error).toBeNull();
  });

  it('TC-UI-022 refresh 失败时记录错误', async () => {
    vi.mocked(api.repo.list).mockRejectedValueOnce(new Error('[E_IO] 无法读取仓库'));
    await store().refresh();
    expect(store().error?.code).toBe('E_IO');
    expect(store().loading).toBe(false);
  });

  it('TC-UI-023 remove 调 repo.remove 并刷新，选中项随之清空', async () => {
    await store().refresh();
    store().select(fakeBackups[0].name);
    vi.mocked(api.repo.list).mockResolvedValueOnce([fakeBackups[1]]);
    await store().remove(fakeBackups[0].name);
    expect(api.repo.remove).toHaveBeenCalledWith(fakeBackups[0].name);
    expect(store().backups).toHaveLength(1);
    expect(store().selected).toBeNull();
  });

  it('TC-UI-024 setRepo 写 config 并刷新列表', async () => {
    await store().setRepo('/new-repo');
    expect(api.config.set).toHaveBeenCalledWith({ repo: '/new-repo' });
    expect(api.repo.list).toHaveBeenCalled();
  });
});
