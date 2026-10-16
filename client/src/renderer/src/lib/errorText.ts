import type { ClientErrorCode } from '@/shared/api';

/** interaction.md §6 的错误码 → 界面文案与建议操作。 */
export interface ErrorText {
  title: string;
  hint: string;
}

const TABLE: Record<ClientErrorCode, ErrorText> = {
  E_USAGE: { title: '参数有误', hint: '检查筛选条件后重试' },
  E_NOT_FOUND: { title: '源目录或备份不存在', hint: '重新选择' },
  E_IO: { title: '读写失败', hint: '检查权限与磁盘空间' },
  E_TARGET_NOT_EMPTY: { title: '还原目录不为空', hint: '选择空目录或新目录' },
  E_CORRUPT: { title: '备份文件已损坏', hint: '无法还原，可删除' },
  E_EXISTS: { title: '同名备份已存在', hint: '修改备份名称' },
  E_AUTH: { title: '口令错误或备份被篡改', hint: '重新输入口令' },
  E_PASSWORD_REQUIRED: { title: '该备份已加密，需要口令', hint: '输入口令' },
  E_CANCELLED: { title: '已取消', hint: '' },
  E_INTERNAL: { title: '内部错误', hint: '请反馈此问题' },
  E_BUSY: { title: '已有任务在运行', hint: '等待或取消当前任务' },
  E_BACKEND_UNAVAILABLE: { title: '后端不可用', hint: '查看状态栏诊断信息' },
};

export function errorText(code: ClientErrorCode, message: string): ErrorText {
  const t = TABLE[code] ?? { title: '错误', hint: '' };
  return { title: message ? `${t.title}：${message}` : t.title, hint: t.hint };
}
