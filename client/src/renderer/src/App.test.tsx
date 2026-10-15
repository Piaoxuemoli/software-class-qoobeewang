import '@testing-library/jest-dom/vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import App from './App';

describe('App', () => {
  it('renders the three tabs', () => {
    window.databackup = { getBackendVersion: vi.fn().mockResolvedValue('0.1.0') };
    render(<App />);
    expect(screen.getByRole('button', { name: '备份' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '管理备份数据' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '备份设置' })).toBeInTheDocument();
  });

  it('shows the backend version in the status bar', async () => {
    window.databackup = { getBackendVersion: vi.fn().mockResolvedValue('0.1.0') };
    render(<App />);
    await waitFor(() =>
      expect(screen.getByTestId('statusbar')).toHaveTextContent('后端版本: 0.1.0'),
    );
  });

  it('shows an error when the backend is unavailable', async () => {
    window.databackup = {
      getBackendVersion: vi.fn().mockRejectedValue(new Error('backend binary not found')),
    };
    render(<App />);
    await waitFor(() => expect(screen.getByTestId('statusbar')).toHaveTextContent('后端不可用'));
  });
});
