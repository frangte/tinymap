import { badge, h, link } from './dom';
import type { Model } from './model';
import type { Problem, Severity } from './problems';

const SEVERITY_NAME: Record<Severity, string> = { error: 'Lỗi', warn: 'Cảnh báo', info: 'Thông tin' };

/** The problems panel, grouped by kind, each with a link to the place. */
export function renderProblems(problems: Problem[]): HTMLElement {
  const root = h('div', { class: 'panel' }, h('h2', {}, `Vấn đề (${problems.length})`));
  if (problems.length === 0) root.append(h('p', {}, 'Không thấy gì.'));
  const groups = new Map<string, Problem[]>();
  for (const p of problems) groups.set(`${p.severity}|${p.kind}`, [...(groups.get(`${p.severity}|${p.kind}`) ?? []), p]);
  for (const [key, list] of groups) {
    const severity = key.split('|')[0] as Severity;
    const section = h('section', { class: 'problem-group' }, h('h3', {}, badge(SEVERITY_NAME[severity], `sev-${severity}`), ` ${list[0].kind} `, h('small', {}, `${list.length}`)));
    const ul = h('ul', {});
    for (const p of list) ul.append(h('li', {}, p.text, ' ', link('→ ' + p.where.text, p.where.hash, 'where')));
    section.append(ul);
    root.append(section);
  }
  return root;
}

/** Every story flag: where it is set, where it is read. */
export function renderFlags(model: Model): HTMLElement {
  const root = h('div', { class: 'panel' }, h('h2', {}, `Cờ truyện (${model.flags.size})`));
  if (model.flags.size === 0) {
    root.append(h('p', { class: 'hint' }, 'Chưa có cờ nào: chưa nội dung nào dùng outcomes, rewards.flags, requires_flags, unless_flags, if_flags hay biến thể "flag:…". Thêm ?fixture vào địa chỉ để xem mẫu.'));
    return root;
  }
  const table = h('table', { class: 'flags' }, h('thead', {}, h('tr', {}, h('th', {}, 'Cờ'), h('th', {}, 'Bật ở'), h('th', {}, 'Đọc ở'))));
  const uses = (list: { where: { text: string; hash: string }; how: string }[]) =>
    h('ul', {}, list.length ? list.map((u) => h('li', {}, link(u.where.text, u.where.hash), h('small', {}, ` · ${u.how}`))) : [h('li', { class: 'bad' }, 'không đâu')]);
  const body = h('tbody', {});
  for (const f of [...model.flags.values()].sort((a, b) => a.id.localeCompare(b.id))) {
    body.append(h('tr', {}, h('td', {}, badge(f.id, 'flag')), h('td', {}, uses(f.sets)), h('td', {}, uses(f.reads))));
  }
  table.append(body);
  root.append(table);
  return root;
}
