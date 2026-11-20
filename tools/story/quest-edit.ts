import { signalSources } from './detail';
import { badge, h, link, type Child } from './dom';
import { chips, iconButton, input, numberInput, row, select, textPair } from './fields';
import type { Model, QuestNode } from './model';
import { questTitle } from './model';
import { addObjective, addOutcome, deleteQuest, dropOutcomes, questReferrers, removeAt, setField, setObjectiveType, setParam, setQuestField, setQuestText } from './ops';
import { FLOWS, ID_PATTERN, KEY_ORDER, LIMITS, OBJECTIVE_TYPES, RAW_PARAM_TYPES, TRIP_ROLES, PLACE_KINDS, TESTS, objectiveType, type ParamDef } from './schema';
import type { Objective, Outcome, QuestDef, QuestText, Rewards } from './types';

export interface QuestEditor {
  commit(change: () => void, silent?: boolean): void;
  /** leaves the quest (after a delete) */
  closeQuest(): void;
}

const section = (title: string, ...body: Child[]): HTMLElement => h('section', { class: 'sect' }, h('h3', {}, title), ...body);

const PARAM_LISTS: Record<ParamDef['kind'], string> = { npc: 'npcs', npcs: 'npcs', place: 'places', signal: 'signals', item: 'items', itemkind: 'itemkinds', emote: 'emotes', fee: 'fees', quest: 'quests', vehicle: 'vehicles', test: 'tests', role: 'roles', placekind: 'placekinds' };

function paramField(ed: QuestEditor, o: Objective, def: ParamDef): HTMLElement {
  const value = o.params?.[def.key];
  const label = `${def.key}${def.required ? '' : ' (tùy chọn)'}`;
  if (def.kind === 'npcs') return row(label, chips(Array.isArray(value) ? (value as string[]) : [], (v) => ed.commit(() => setParam(o, def.key, v)), { list: 'npcs' }));
  const fixed: Record<string, string[]> = { role: TRIP_ROLES, test: TESTS, placekind: PLACE_KINDS };
  if (fixed[def.kind]) return row(label, select([{ value: '', label: '(không)' }, ...fixed[def.kind].map((x) => ({ value: x }))], String(value ?? ''), (v) => ed.commit(() => setParam(o, def.key, v))));
  return row(label, input(String(value ?? ''), (v) => ed.commit(() => setParam(o, def.key, v)), { list: PARAM_LISTS[def.kind] as 'npcs', mono: true }));
}

/** The form of one objective (or of an outcome's objective part). */
function objectiveForm(model: Model, ed: QuestEditor, q: QuestDef, o: Objective, order: readonly string[]): HTMLElement {
  const ids = [...q.objectives, ...(q.outcomes ?? [])].filter((x) => x !== o).map((x) => x.id);
  const type = objectiveType(o.type);
  const box = h('div', { class: 'oform' });
  box.append(
    row('id', input(o.id ?? '', (v) => ed.commit(() => setQuestField(o, 'id', v, order)), { mono: true, pattern: ID_PATTERN }), ids.includes(o.id) ? 'id này trùng' : 'Cố định: tiến độ lưu theo id'),
    row('loại', select(OBJECTIVE_TYPES.map((t) => ({ value: t.type, label: `${t.name} (${t.type})` })), o.type, (v) => ed.commit(() => setObjectiveType(o, v)))),
  );
  if (RAW_PARAM_TYPES.includes(o.type)) {
    const raw = h('textarea', { class: 'fld area mono', rows: 4, spellcheck: false });
    raw.value = JSON.stringify(o.params ?? {}, null, 2);
    raw.addEventListener('change', () => {
      try {
        const parsed = JSON.parse(raw.value) as Record<string, unknown>;
        raw.classList.remove('invalid');
        ed.commit(() => {
          o.params = parsed;
        });
      } catch {
        raw.classList.add('invalid');
      }
    });
    box.append(row('params (JSON)', raw, 'stops, seconds, order: xem server/internal/quest/objective/errand.go'));
  } else for (const def of type?.params ?? []) box.append(paramField(ed, o, def));
  box.append(
    row(type?.count ? `số lượng (${type.count})` : 'số lượng', numberInput(o.count, (n) => ed.commit(() => setQuestField(o, 'count', n, order)), '1')),
    row('chữ trên sổ', textPair(o.text, (lang, v) => ed.commit(() => ((o.text ??= {})[lang] = v), true), LIMITS.objectiveText)),
    row('chỉ tới', input(o.at ?? '', (v) => ed.commit(() => setQuestField(o, 'at', v, order)), { list: 'npcs', mono: true }), 'Mũi tên dẫn đường: một nơi hay NPC'),
  );
  const sources = signalSources(model, o);
  if (sources) box.append(sources);
  return box;
}

function messages(ed: QuestEditor, list: QuestText[] | undefined, make: (next: QuestText[]) => void): HTMLElement {
  const box = h('div', { class: 'msgs' });
  (list ?? []).forEach((m, i) => {
    box.append(
      h(
        'div',
        { class: 'msg' },
        row('từ', input(m.from, (v) => ed.commit(() => (m.from = v)), { list: 'contacts', mono: true })),
        textPair(m.text, (lang, v) => ed.commit(() => ((m.text ??= {})[lang] = v), true), LIMITS.message),
        iconButton('× bỏ tin', 'Bỏ tin nhắn này', () => ed.commit(() => make((list ?? []).filter((_, j) => j !== i))), 'text danger'),
      ),
    );
  });
  if ((list?.length ?? 0) < LIMITS.messages) {
    const add = h('button', { class: 'btn small', type: 'button' }, '+ tin nhắn');
    add.addEventListener('click', () => ed.commit(() => make([...(list ?? []), { from: '', text: { vi: '', en: '' } }])));
    box.append(add);
  }
  return box;
}

/** `prune`: the owner's rewards object is dropped again when nothing is left in it (an outcome's). */
function rewardsForm(model: Model, ed: QuestEditor, owner: { rewards?: Rewards }, flagsHere: boolean, prune: boolean): HTMLElement {
  const r = owner.rewards ?? {};
  const set = (key: string, value: unknown) =>
    ed.commit(() => {
      const target = (owner.rewards ??= {});
      setField(target, key, value, KEY_ORDER.rewards);
      if (prune && Object.keys(target).length === 0) delete owner.rewards;
    });
  const titles = (model.content.quests as unknown as { titles?: { id: string }[] }).titles;
  const box = h('div', { class: 'rform' });
  box.append(
    row('đồng (xu)', numberInput(r.xu, (n) => set('xu', n))),
    row('danh hiệu', chips(r.titles ?? [], (v) => set('titles', v), { list: 'titles', known: (x) => titles?.some((t) => t.id === x) ?? true })),
    row('tem', chips(r.stamps ?? [], (v) => set('stamps', v), { list: 'stamps' })),
    row('quyền', input(r.capability ?? '', (v) => set('capability', v), { list: 'caps', mono: true })),
  );
  if (flagsHere) box.append(row('bật cờ', chips(r.flags ?? [], (v) => set('flags', v), { list: 'flags', max: LIMITS.flags })));
  return box;
}

function outcomeForm(model: Model, ed: QuestEditor, q: QuestDef, o: Outcome, i: number): HTMLElement {
  const box = h('div', { class: 'outcome edit' });
  const tools = h('div', { class: 'ctools' }, iconButton('×', 'Xóa kết cục', () => ed.commit(() => removeAt(q.outcomes!, i))));
  box.append(h('div', { class: 'ohead' }, h('b', {}, o.title?.vi || o.id || `kết cục ${i + 1}`), badge(`${q.id}:${o.id}`, 'flag', 'Cờ tự bật khi kết thúc bằng kết cục này'), tools));
  box.append(row('tên (hiện trong sổ)', textPair(o.title, (lang, v) => ed.commit(() => ((o.title ??= {})[lang] = v), true), LIMITS.title * 2)));
  box.append(objectiveForm(model, ed, q, o, KEY_ORDER.outcome));
  box.append(row('bật cờ', chips(o.flags ?? [], (v) => ed.commit(() => setQuestField(o, 'flags', v, KEY_ORDER.outcome)), { list: 'flags', max: LIMITS.flags })));
  box.append(h('h4', {}, 'Thưởng riêng của kết cục'), rewardsForm(model, ed, o, false, true));
  const done = o.texts?.on_done;
  box.append(
    h('h4', {}, 'Tin nhắn khi kết thúc bằng kết cục này'),
    messages(ed, done, (next) => {
      if (next.length) {
        o.texts ??= {};
        o.texts.on_done = next;
      } else delete o.texts;
    }),
  );
  return box;
}

function referrersWarning(content: Model['content'], id: string): string {
  const refs = questReferrers(content, id);
  if (refs.length === 0) return 'Không nơi nào nhắc tới quest này.';
  return `Những nơi đang nhắc tới (sẽ thành nối hỏng):\n${refs.map((r) => `  ${r.where}`).join('\n')}`;
}

/** The side panel of a quest in edit mode. */
export function renderQuestEditor(model: Model, ed: QuestEditor, q: QuestNode): HTMLElement {
  const def = q.def;
  const real = model.content.quests.quests.find((x) => x.id === def.id);
  if (!real) return h('aside', { class: 'detail' }, h('p', { class: 'hint' }, 'Thành tích: chưa sửa được trong bản này.'));
  const daily = real.repeat === 'daily';
  const panel = h('aside', { class: 'detail editing' });
  const del = h('button', { class: 'btn danger small', type: 'button' }, 'Xóa quest');
  del.addEventListener('click', () => {
    if (!window.confirm(`Xóa quest "${real.id}"?\n\n${referrersWarning(model.content, real.id)}`)) return;
    ed.commit(() => deleteQuest(model.content, real.id));
    ed.closeQuest();
  });
  panel.append(
    h('div', { class: 'detail-head' }, h('a', { class: 'close', href: '#/', title: 'Đóng' }, '×'), h('h2', {}, questTitle(real)), h('div', { class: 'id' }, `${real.id} · ${real.questline}`), del),
    section('Tên và mô tả', row('tên', textPair(real.title, (lang, v) => ed.commit(() => setQuestText(real, 'title', lang, v), true), LIMITS.questTitle)), row('mô tả', textPair(real.about, (lang, v) => ed.commit(() => setQuestText(real, 'about', lang, v), true), LIMITS.about))),
  );
  const exists = (id: string) => model.quests.has(id);
  const sets = (key: string) => (v: string[]) => ed.commit(() => setQuestField(real, key, v));
  panel.append(
    section(
      'Điều kiện',
      row('cần xong quest', chips(real.requires ?? [], sets('requires'), { list: 'quests', known: exists }), 'requires'),
      row('cần cờ', daily ? h('span', { class: 'hint' }, 'quest hằng ngày không đọc cờ') : chips(real.requires_flags ?? [], sets('requires_flags'), { list: 'flags', max: LIMITS.flags }), 'requires_flags'),
      row('không được có cờ', daily ? h('span', { class: 'hint' }, '—') : chips(real.unless_flags ?? [], sets('unless_flags'), { list: 'flags', max: LIMITS.flags }), 'unless_flags'),
      row('mời xuất phát (tags)', chips(real.tags_any ?? [], sets('tags_any'), { list: 'tags' }), 'tags_any: bỏ trống là mời mọi người'),
      row('cần quyền', chips(real.requires_caps ?? [], sets('requires_caps'), { list: 'caps' })),
      row('không mời nếu có quyền', chips(real.unless_caps ?? [], sets('unless_caps'), { list: 'caps' })),
    ),
  );
  const objs = h('div', {});
  real.objectives.forEach((o, i) => {
    const tools = h('div', { class: 'ctools' }, iconButton('×', 'Xóa mục tiêu', () => ed.commit(() => removeAt(real.objectives, i))));
    objs.append(h('div', { class: 'outcome edit' }, h('div', { class: 'ohead' }, h('b', {}, `${i + 1}. ${o.id ?? ''}`), tools), objectiveForm(model, ed, real, o, KEY_ORDER.objective)));
  });
  const addO = h('button', { class: 'btn small', type: 'button', disabled: real.objectives.length >= LIMITS.objectives }, '+ mục tiêu');
  addO.addEventListener('click', () => ed.commit(() => addObjective(real)));
  panel.append(section('Mục tiêu', row('thứ tự', select(FLOWS.map((value) => ({ value })), real.flow ?? 'sequence', (v) => ed.commit(() => setQuestField(real, 'flow', v)), 'sequence: lần lượt; all: đủ hết; any: một cái là xong')), objs, addO));
  const outs = h('div', {});
  (real.outcomes ?? []).forEach((o, i) => outs.append(outcomeForm(model, ed, real, o, i)));
  const addOut = h('button', { class: 'btn small', type: 'button', disabled: daily || (real.outcomes?.length ?? 0) >= LIMITS.outcomesMax }, '+ kết cục');
  addOut.addEventListener('click', () => ed.commit(() => (real.outcomes?.length ? addOutcome(real) : (addOutcome(real), addOutcome(real)))));
  const dropAll = real.outcomes?.length ? h('button', { class: 'btn small', type: 'button' }, 'Bỏ hết kết cục') : null;
  dropAll?.addEventListener('click', () => ed.commit(() => dropOutcomes(real)));
  panel.append(
    section('Kết cục', h('p', { class: 'hint' }, daily ? 'Quest hằng ngày không có kết cục.' : `${LIMITS.outcomesMin} đến ${LIMITS.outcomesMax} kết cục; xong các mục tiêu thì mọi kết cục cùng mở, cái nào xong trước kết thúc quest. Id kết cục dùng chung với id mục tiêu.`), outs, addOut, dropAll),
  );
  panel.append(section('Phần thưởng khi xong quest', rewardsForm(model, ed, real, !daily, false)));
  panel.append(
    section(
      'Tin nhắn',
      h('h4', {}, 'Khi bắt đầu'),
      messages(ed, real.texts?.on_start, (next) => setTexts(real, 'on_start', next)),
      h('h4', {}, 'Khi xong'),
      messages(ed, real.texts?.on_done, (next) => setTexts(real, 'on_done', next)),
    ),
  );
  const next = model.edges.filter((e) => e.from === real.id);
  if (next.length) panel.append(section('Mở ra', h('ul', {}, next.map((e) => h('li', {}, link(e.to, `#/quest/${e.to}`))))));
  return panel;
}

function setTexts(q: QuestDef, phase: 'on_start' | 'on_done', next: QuestText[]): void {
  const texts = (q.texts ??= {});
  if (next.length) texts[phase] = next;
  else delete texts[phase];
  if (Object.keys(texts).length === 0) delete q.texts;
}
