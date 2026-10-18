/**
 * 反证：容器内以 root 运行 Electron 是否**必须**加 --no-sandbox。
 * 由 run-probe-in-docker.sh 调用；不录视频，只启动→关窗。
 */
import path from 'node:path';
import { _electron as electron } from '@playwright/test';

const clientDir = path.resolve(import.meta.dirname, '..', '..');
const repoDir = path.resolve(clientDir, '..');
const HOME = '/home/demo';
const backendBin = path.join(repoDir, 'backend', 'bin', 'databackup');

try {
  const app = await electron.launch({
    args: ['--force-device-scale-factor=2', '.'],
    cwd: clientDir,
    env: {
      ...process.env,
      HOME,
      XDG_CONFIG_HOME: `${HOME}/.config`,
      DATABACKUP_BIN: backendBin,
    },
    timeout: 25_000,
  });
  console.log('RESULT: launched WITHOUT --no-sandbox (root sandbox did not block)');
  await app.close();
} catch (e) {
  const msg = String(e);
  const fatal = msg
    .split('\n')
    .filter((l) => /sandbox|FATAL|SUID/i.test(l))
    .slice(0, 5);
  console.log('RESULT: launch WITHOUT --no-sandbox failed → --no-sandbox 是必需的');
  console.log(fatal.length > 0 ? fatal.join('\n') : msg.slice(0, 600));
}
