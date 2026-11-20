import { badge, h } from './dom';
import { chips, iconButton, input, numberInput, row, select, textPair } from './fields';
import { addChoice, addNode, addVariant, deleteChoice, deleteNode, moveChoice, nodeReferrers, removeVariant, setCall, setChoiceField, setChoiceText, setGotoIf, setNodeField, setSay, setStart } from './ops';
import { DEFAULT_VARIANT, ID_PATTERN, LIMITS, SIGNAL_PATTERN } from './schema';
import { DIALOGUE_END, type Choice, type DialogueDef, type DialogueNodeDef } from './types';
import type { Model } from './model';

/** What the dialogue view needs to change the draft: `commit` runs one change as one undo step. */
export interface DialogueEditor {
  commit(change: () => void, silent?: boolean): void;
}

const EMOTES = ['wave', 'bow', 'photo', 'exercise'];

const gotoOptions = (def: DialogueDef) => [{ value: DIALOGUE_END, label: '→ kết thúc' }, ...Object.keys(def.nodes).map((id) => ({ value: id, label: `→ ${id}` }))];

/** The editable lines of a node: one pair of fields per variant, and a field to add a variant. */
function sayEditor(ed: DialogueEditor, node: DialogueNodeDef): HTMLElement {
  const box = h('div', { class: 'say-edit' });
  for (const key of Object.keys(node.say)) {
    const title = h('div', { class: 'vhead' }, badge(key === DEFAULT_VARIANT ? 'mặc định' : key, key.startsWith('flag:') ? 'flag' : key === DEFAULT_VARIANT ? 'mono' : 'cap'));
    if (key !== DEFAULT_VARIANT) title.append(iconButton('×', 'Bỏ biến thể này', () => ed.commit(() => removeVariant(node, key))));
    box.append(title, textPair(node.say[key], (lang, v) => ed.commit(() => setSay(node, key, lang, v), true), LIMITS.say));
  }
  const add = input('', (v) => ed.commit(() => addVariant(node, v)), { list: 'variants', placeholder: '+ biến thể: xuất phát, quyền hay flag:<cờ>', title: 'Một câu khác cho người có xuất phát, quyền hay cờ này', mono: true });
  box.append(add);
  return box;
}

function nodeProps(ed: DialogueEditor, model: Model, node: DialogueNodeDef): HTMLElement {
  const items = model.content.items;
  const opt = (xs: string[]) => [{ value: '', label: '(không)' }, ...xs.map((value) => ({ value }))];
  return h(
    'details',
    { class: 'props' },
    h('summary', {}, 'Ô này: điệu bộ, shop, phí, tín hiệu'),
    row('điệu bộ', select(opt(EMOTES), node.emote ?? '', (v) => ed.commit(() => setNodeField(node, 'emote', v)))),
    row('shop', select(opt((items.shops ?? []).map((s) => s.id)), node.shop ?? '', (v) => ed.commit(() => setNodeField(node, 'shop', v)))),
    row('phí', select(opt((items.fees ?? []).map((s) => s.id)), node.fee ?? '', (v) => ed.commit(() => setNodeField(node, 'fee', v)))),
    row('gửi tín hiệu', input(node.emit ?? '', (v) => ed.commit(() => setNodeField(node, 'emit', v)), { list: 'signals', mono: true, pattern: SIGNAL_PATTERN }), 'Tự gửi khi cuộc nói chuyện tới ô này'),
    row('xóa điểm', h('input', { type: 'checkbox', checked: node.reset_marks === true, onchange: (ev: Event) => ed.commit(() => setNodeField(node, 'reset_marks', (ev.target as HTMLInputElement).checked)) })),
  );
}

function choiceConditions(ed: DialogueEditor, def: DialogueDef, c: Choice): HTMLElement {
  const set = (key: Parameters<typeof setChoiceField>[1]) => (v: string | string[]) => ed.commit(() => setChoiceField(c, key, v));
  const flagKnown = () => true;
  return h(
    'details',
    { class: 'props' },
    h('summary', {}, 'Điều kiện và tín hiệu'),
    row('gửi tín hiệu', input(c.emit ?? '', set('emit') as (v: string) => void, { list: 'signals', mono: true, pattern: SIGNAL_PATTERN }), 'Để xong một mục tiêu quest đang chờ'),
    row('cần quyền', chips(c.requires ?? [], set('requires') as (v: string[]) => void, { list: 'caps', known: flagKnown })),
    row('trừ quyền', chips(c.unless ?? [], set('unless') as (v: string[]) => void, { list: 'caps' })),
    row('đang làm quest', input(c.requires_quest ?? '', set('requires_quest') as (v: string) => void, { list: 'quests', mono: true }), 'Chỉ hiện khi quest này đang chạy'),
    row('cần cờ', chips(c.if_flags ?? [], set('if_flags') as (v: string[]) => void, { list: 'flags', max: LIMITS.flags })),
    row('không cờ', chips(c.unless_flags ?? [], set('unless_flags') as (v: string[]) => void, { list: 'flags', max: LIMITS.flags })),
    row('cộng điểm', input(c.mark ?? '', set('mark') as (v: string) => void, { mono: true, pattern: SIGNAL_PATTERN }), 'Mỗi lần chọn câu này cộng một điểm vào bảng điểm của cuộc nói chuyện'),
    c.mark
      ? row(
          'nếu đủ điểm',
          h(
            'span',
            { class: 'inline' },
            numberInput(c.goto_if?.at_least, (n) => ed.commit(() => setGotoIf(c, n ? { mark: c.mark!, at_least: n, goto: c.goto_if?.goto ?? c.goto } : undefined)), '≥'),
            select(gotoOptions(def), c.goto_if?.goto ?? c.goto, (v) => ed.commit(() => setGotoIf(c, { mark: c.mark!, at_least: c.goto_if?.at_least ?? 1, goto: v }))),
          ),
        )
      : null,
  );
}

function choiceRow(ed: DialogueEditor, def: DialogueDef, node: DialogueNodeDef, c: Choice, i: number): HTMLElement {
  const tools = h(
    'div',
    { class: 'ctools' },
    iconButton('↑', 'Lên', () => ed.commit(() => moveChoice(node, i, -1))),
    iconButton('↓', 'Xuống', () => ed.commit(() => moveChoice(node, i, 1))),
    iconButton('×', 'Xóa câu trả lời', () => ed.commit(() => deleteChoice(node, i))),
  );
  const handle = h('span', { class: 'handle', title: 'Kéo tới một ô (hay tới "kết thúc") để nối', 'data-choice': i }, '●');
  const target = select(gotoOptions(def), c.goto, (v) => ed.commit(() => setChoiceField(c, 'goto', v)), 'Đi tới ô nào');
  target.classList.add('goto');
  return h(
    'li',
    { class: 'answer editing', 'data-choice': i },
    h('div', { class: 'ahead' }, h('b', {}, `${i + 1}.`), target, tools, handle),
    textPair(c.text, (lang, v) => ed.commit(() => setChoiceText(c, lang, v), true), LIMITS.choice),
    choiceConditions(ed, def, c),
  );
}

/** The body of a node box in edit mode (the head is drawn by the view). */
export function editableBody(model: Model, ed: DialogueEditor, def: DialogueDef, node: DialogueNodeDef): HTMLElement[] {
  const answers = h('ol', { class: 'answers' });
  (node.choices ?? []).forEach((c, i) => answers.append(choiceRow(ed, def, node, c, i)));
  const add = h('button', { class: 'btn small', type: 'button', disabled: (node.choices?.length ?? 0) >= LIMITS.choices }, '+ câu trả lời');
  add.addEventListener('click', () => ed.commit(() => addChoice(def, node)));
  return [sayEditor(ed, node), nodeProps(ed, model, node), answers, add];
}

/** The buttons in a node's head: make it the start, the call node, delete it. */
export function nodeTools(ed: DialogueEditor, def: DialogueDef, id: string): HTMLElement {
  const tools = h('span', { class: 'ntools' });
  if (id !== def.start) tools.append(iconButton('Bắt đầu', 'Đặt làm ô bắt đầu cuộc nói chuyện', () => ed.commit(() => setStart(def, id)), 'text'));
  if (id !== def.call) tools.append(iconButton('Gọi điện', 'Đặt làm ô khi NPC được gọi điện', () => ed.commit(() => setCall(def, id)), 'text'));
  else tools.append(iconButton('Bỏ gọi', 'Không dùng ô này cho cuộc gọi', () => ed.commit(() => setCall(def, '')), 'text'));
  tools.append(
    iconButton('Xóa', 'Xóa ô này', () => {
      const refs = nodeReferrers(def, id);
      const lines = refs.map((r) => `  ${r.node}, câu trả lời ${r.choice + 1}`).join('\n');
      const warn = refs.length ? `\n\nCác câu trả lời đang nối tới ô này (sẽ thành nối hỏng, đỏ):\n${lines}` : '';
      const extra = id === def.start ? '\n\nĐây là ô bắt đầu.' : '';
      if (window.confirm(`Xóa ô "${id}"?${warn}${extra}`)) ed.commit(() => deleteNode(def, id));
    }, 'text danger'),
  );
  return tools;
}

/** Above the graph: add a node, choose the start and call nodes. */
export function dialogueToolbar(ed: DialogueEditor, def: DialogueDef): HTMLElement {
  const nodeIds = Object.keys(def.nodes);
  const name = h('input', { type: 'text', class: 'fld', placeholder: 'id ô mới (chữ thường, số, _)', 'data-testid': 'new-node-id', autocomplete: 'off' });
  const add = h('button', { class: 'btn', type: 'button' }, '+ Ô mới');
  const create = () => {
    const id = name.value.trim();
    if (!ID_PATTERN.test(id) || id === DIALOGUE_END || id in def.nodes) {
      name.classList.add('invalid');
      return;
    }
    ed.commit(() => addNode(def, id));
  };
  add.addEventListener('click', create);
  name.addEventListener('keydown', (ev) => ev.key === 'Enter' && create());
  const start = select(nodeIds.map((value) => ({ value })), def.start, (v) => ed.commit(() => setStart(def, v)), 'Ô bắt đầu');
  const call = select([{ value: '', label: '(không có)' }, ...nodeIds.map((value) => ({ value }))], def.call ?? '', (v) => ed.commit(() => setCall(def, v)), 'Ô khi được gọi điện');
  return h('div', { class: 'dtoolbar' }, name, add, h('label', {}, 'bắt đầu ', start), h('label', {}, 'gọi điện ', call), h('span', { class: 'hint' }, 'Kéo dấu ● của câu trả lời tới một ô để nối.'));
}
