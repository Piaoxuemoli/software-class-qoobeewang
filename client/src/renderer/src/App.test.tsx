import '@testing-library/jest-dom/vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';
import { initialAppState, useAppStore } from './stores/app';

beforeEach(() => {
  useAppStore.setState(initialAppState);
});

function mockApi(overrides: Partial<Window['databackup']> = {}): void {
  window.databackup = {
    getBackendVersion: vi.fn().mockResolvedValue('0.1.0'),
    window: {
      minimize: vi.fn(),
      toggleMaximize: vi.fn(),
      close: vi.fn(),
      onMaximizedChange: vi.fn().mockReturnValue(() => {}),
    },
    ...overrides,
  };
}

describe('App', () => {
  it('renders the sidebar navigation', () => {
    mockApi();
    render(<App />);
    expect(screen.getByRole('button', { name: '备份' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '备份管理' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument();
  });

  it('renders the frameless title bar with window controls', () => {
    mockApi();
    render(<App />);
    expect(screen.getByRole('button', { name: '最小化' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '最大化' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '关闭' })).toBeInTheDocument();
  });

  it('routes window control clicks through window.databackup.window', () => {
    mockApi();
    render(<App />);
    fireEvent.click(screen.getByRole('button', { name: '最小化' }));
    expect(window.databackup.window.minimize).toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '关闭' }));
    expect(window.databackup.window.close).toHaveBeenCalled();
  });

  it('shows the backend version in the status bar', async () => {
    mockApi();
    render(<App />);
    await waitFor(() =>
      expect(screen.getByTestId('statusbar')).toHaveTextContent('后端版本: 0.1.0'),
    );
  });

  it('shows an error when the backend is unavailable', async () => {
    mockApi({
      getBackendVersion: vi.fn().mockRejectedValue(new Error('backend binary not found')),
    });
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('statusbar')).toHaveTextContent('后端不可用'));
  });

  it('switches pages via the sidebar', () => {
    mockApi();
    render(<App />);
    fireEvent.click(screen.getByRole('button', { name: '设置' }));
    expect(useAppStore.getState().page).toBe('settings');
    expect(screen.getByRole('switch', { name: '减少动效' })).toBeInTheDocument();
  });

  it('starts on the backup page regardless of earlier tests', () => {
    mockApi();
    render(<App />);
    expect(screen.getByRole('button', { name: '备份' })).toHaveAttribute('aria-current', 'page');
  });

  it('collapses the sidebar to icons with Ctrl+B and expands it again', () => {
    mockApi();
    render(<App />);
    const sidebar = screen.getByTestId('sidebar');
    expect(sidebar).toHaveAttribute('data-collapsed', 'false');

    fireEvent.keyDown(window, { key: 'b', ctrlKey: true });
    expect(sidebar).toHaveAttribute('data-collapsed', 'true');
    expect(sidebar).toHaveClass('w-16');
    // 折叠后导航仍可用（按钮保留可访问名称）。
    fireEvent.click(screen.getByRole('button', { name: '备份管理' }));
    expect(useAppStore.getState().page).toBe('manage');

    fireEvent.keyDown(window, { key: 'B', metaKey: true });
    expect(sidebar).toHaveAttribute('data-collapsed', 'false');

    fireEvent.keyDown(window, { key: 'b', ctrlKey: true, shiftKey: true });
    expect(sidebar).toHaveAttribute('data-collapsed', 'false');
  });
});
