// mud.ts — gl1tch-mud HUD client

interface ServerMsg {
  type: string;
  payload?: any;
}

interface StateUpdate {
  hp: number;
  maxHp: number;
  roomName: string;
  exits: string[];
  inventory: InvItem[];
  credits: number;
  recipes?: Recipe[];
  room_npcs?: RoomNPCInfo[];
  room_items?: RoomItemInfo[];
  room_resources?: RoomResourceInfo[];
  quests?: QuestInfo[];
  skills?: SkillInfo[];
}

interface InvItem {
  id: string;
  name: string;
  desc: string;
  tier: string;
}

interface RecipeIngredient {
  id: string;
  count: number;
}

interface Recipe {
  id: string;
  name: string;
  ingredients: RecipeIngredient[];
  outputId: string;
  outputName: string;
  skillReq?: number;
}

interface RoomNPCInfo {
  id: string;
  name: string;
  can_talk: boolean;
  can_trade: boolean;
  attackable: boolean;
}

interface RoomItemInfo {
  id: string;
  name: string;
  takeable: boolean;
}

interface RoomResourceInfo {
  id: string;
  name: string;
  action: string;
}

interface QuestInfo {
  id: string;
  title: string;
  description?: string;
  obj_count: number;
  obj_progress: number;
}

interface SkillInfo {
  name: string;
  level: number;
  xp: number;
}

// ── ANSI → HTML ──────────────────────────────────────────────────────────────

const ANSI_OPEN: Record<string, string> = {
  '1':  'font-weight:bold',
  '31': 'color:#ff5555',
  '32': 'color:#50fa7b',
  '33': 'color:#f1fa8c',
  '34': 'color:#bd93f9',
  '35': 'color:#ff79c6',
  '36': 'color:#8be9fd',
  '37': 'color:#f8f8f2',
};

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function ansiToHtml(text: string): string {
  const parts = text.split(/(\x1b\[[0-9;]*m)/);
  let openCount = 0;
  let out = '';

  for (const part of parts) {
    const m = part.match(/^\x1b\[([0-9;]*)m$/);
    if (m) {
      const codes = m[1].split(';');
      for (const code of codes) {
        if (code === '0' || code === '') {
          out += '</span>'.repeat(openCount);
          openCount = 0;
        } else if (ANSI_OPEN[code]) {
          out += `<span style="${ANSI_OPEN[code]}">`;
          openCount++;
        }
      }
    } else {
      out += escapeHtml(part);
    }
  }

  out += '</span>'.repeat(openCount);
  return out;
}

// ── Tier icons & colours ─────────────────────────────────────────────────────

const TIER_ICON: Record<string, string> = {
  'noise':    '◦',
  'signal':   '◆',
  'ghost':    '◈',
  'zero-day': '⚡',
  'flatline': '☠',
};

// ── World-specific action buttons ────────────────────────────────────────────

interface ActionDef {
  cmd?: string;
  icon: string;
  label: string;
  special?: string;
  cls?: string;
}

const WORLD_ACTIONS: Record<string, ActionDef[]> = {
  cyberspace: [
    { icon: '👁',  label: 'Look',    cmd: 'look' },
    { icon: '⚔️', label: 'Attack',  cmd: 'attack' },
    { icon: '💻',  label: 'Hack',    cmd: 'hack' },
    { icon: '🔍',  label: 'Search',  cmd: 'search' },
    { icon: '🗺',  label: 'Explore', cmd: 'explore' },
    { icon: '⚡',  label: 'Skills',  cmd: 'skills' },
    { icon: '📋',  label: 'Quests',  cmd: 'quests' },
    { icon: '🔧',  label: 'Craft',   special: 'craft', cls: 'craft-btn' },
  ],
  blockhaven: [
    { icon: '👁',  label: 'Look',    cmd: 'look' },
    { icon: '⚔️', label: 'Attack',  cmd: 'attack' },
    { icon: '🌿',  label: 'Forage',  cmd: 'forage' },
    { icon: '🔍',  label: 'Search',  cmd: 'search' },
    { icon: '🗺',  label: 'Explore', cmd: 'explore' },
    { icon: '⚡',  label: 'Skills',  cmd: 'skills' },
    { icon: '📋',  label: 'Quests',  cmd: 'quests' },
    { icon: '🔧',  label: 'Craft',   special: 'craft', cls: 'craft-btn' },
  ],
};

const DEFAULT_ACTIONS: ActionDef[] = WORLD_ACTIONS['cyberspace'];

// ── HP hearts ────────────────────────────────────────────────────────────────

function renderHearts(hp: number, maxHP: number): string {
  const total = 5;
  const pct   = maxHP > 0 ? hp / maxHP : 0;
  const filled = Math.round(pct * total);
  let color = '#50fa7b';
  if (pct <= 0.5)  color = '#f1fa8c';
  if (pct <= 0.25) color = '#ff5555';

  let html = '';
  for (let i = 0; i < total; i++) {
    const c = i < filled ? color : '#44475a';
    html += `<span style="color:${c}">&#9829;</span>`;
  }
  return html;
}

// ── Inventory grid ───────────────────────────────────────────────────────────

const SLOTS = 8;

function renderInventory(items: InvItem[], onItemClick: (item: InvItem) => void) {
  const grid = document.getElementById('inv-grid');
  if (!grid) return;

  while (grid.firstChild) grid.removeChild(grid.firstChild);

  for (let i = 0; i < SLOTS; i++) {
    const slot = document.createElement('div');
    slot.className = 'inv-slot';

    if (i < items.length) {
      const item = items[i];
      slot.classList.add('occupied');
      slot.dataset.tier = item.tier || 'noise';
      slot.draggable = true;

      const icon = document.createElement('div');
      icon.className = 'slot-icon';
      icon.textContent = TIER_ICON[item.tier] ?? '◦';

      const label = document.createElement('div');
      label.className = 'slot-label';
      const short = item.name.length > 10 ? item.name.slice(0, 9) + '…' : item.name;
      label.textContent = short;

      slot.appendChild(icon);
      slot.appendChild(label);
      slot.title = `${item.name}\n${item.desc} — drag to craft grid`;

      const captured = item;
      slot.addEventListener('click', () => onItemClick(captured));

      // Drag start — serialise item into transfer data.
      slot.addEventListener('dragstart', (e) => {
        e.dataTransfer!.effectAllowed = 'copy';
        e.dataTransfer!.setData('application/gl1tch-item', JSON.stringify(captured));
        slot.classList.add('dragging');
      });
      slot.addEventListener('dragend', () => slot.classList.remove('dragging'));
    } else {
      slot.style.opacity = '0.25';
    }

    grid.appendChild(slot);
  }
}

// ── Compass ──────────────────────────────────────────────────────────────────

const DIR_BUTTONS: Record<string, string> = {
  north: 'btn-n',
  south: 'btn-s',
  east:  'btn-e',
  west:  'btn-w',
};

function updateCompass(exits: string[]) {
  const exitSet = new Set(exits.map(e => e.toLowerCase()));
  for (const [dir, id] of Object.entries(DIR_BUTTONS)) {
    const btn = document.getElementById(id);
    if (!btn) continue;
    if (exitSet.has(dir)) {
      btn.classList.add('active');
    } else {
      btn.classList.remove('active');
    }
  }
}

// ── Kids mode ────────────────────────────────────────────────────────────────

function applyKidsMode(): void {
  _kidsMode = true;
  document.body.dataset.ui = 'kids';
  // Restore saved input visibility preference
  const INPUT_OPEN_KEY = 'bh-kids-input-open';
  const savedOpen = localStorage.getItem(INPUT_OPEN_KEY) === 'true';
  // applyInputVisibility is defined inside initMUD; call the DOM directly here
  const cmdEl    = document.getElementById('cmd-input');
  const sendEl   = document.getElementById('send-btn');
  const promptEl = document.querySelector<HTMLElement>('.prompt');
  const toggle   = document.getElementById('kids-input-toggle');
  if (savedOpen) {
    cmdEl?.classList.add('kids-visible');
    sendEl?.classList.add('kids-visible');
    promptEl?.classList.add('kids-visible');
    if (toggle) toggle.style.opacity = '1';
    (cmdEl as HTMLInputElement | null)?.focus();
  } else {
    cmdEl?.classList.remove('kids-visible');
    sendEl?.classList.remove('kids-visible');
    promptEl?.classList.remove('kids-visible');
    if (toggle) toggle.style.opacity = '0.45';
  }
}

function formatResourceName(id: string): string {
  return id.replace(/[-_]/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}

function rebuildRoomContext(state: StateUpdate): void {
  const npcList = document.getElementById('room-npcs-list');
  if (npcList) {
    while (npcList.firstChild) npcList.removeChild(npcList.firstChild);
    const npcs = state.room_npcs ?? [];
    if (npcs.length === 0) {
      const empty = document.createElement('div');
      empty.style.cssText = 'font-size:0.7rem;color:var(--comment)';
      empty.textContent = 'Nobody here.';
      npcList.appendChild(empty);
    } else {
      for (const npc of npcs) {
        const row = document.createElement('div');
        row.className = 'room-npc-row';

        const nameEl = document.createElement('span');
        nameEl.textContent = npc.name;

        const badges = document.createElement('span');
        badges.className = 'room-npc-badges';
        if (npc.can_talk) {
          const b = document.createElement('span');
          b.className = 'room-npc-badge';
          b.title = 'Can talk';
          b.textContent = '💬';
          badges.appendChild(b);
        }
        if (npc.can_trade) {
          const b = document.createElement('span');
          b.className = 'room-npc-badge';
          b.title = 'Can trade';
          b.textContent = '🛒';
          badges.appendChild(b);
        }
        if (npc.attackable) {
          const b = document.createElement('span');
          b.className = 'room-npc-badge';
          b.title = 'Hostile';
          b.textContent = '⚔️';
          badges.appendChild(b);
        }
        row.appendChild(nameEl);
        row.appendChild(badges);
        npcList.appendChild(row);
      }
    }
  }

  const groundEl = document.getElementById('room-ground-list');
  if (groundEl) {
    while (groundEl.firstChild) groundEl.removeChild(groundEl.firstChild);
    const items = state.room_items ?? [];
    if (items.length === 0) {
      const empty = document.createElement('div');
      empty.style.cssText = 'font-size:0.7rem;color:var(--comment)';
      empty.textContent = 'Nothing here.';
      groundEl.appendChild(empty);
    } else {
      for (const item of items) {
        const row = document.createElement('div');
        row.className = 'room-ground-row';
        const nameEl = document.createElement('span');
        nameEl.textContent = item.name;
        row.appendChild(nameEl);
        if (item.takeable) {
          const btn = document.createElement('button');
          btn.type = 'button';
          btn.className = 'kids-take-btn';
          btn.textContent = 'Take';
          const capturedId = item.id;
          btn.addEventListener('click', () => _dispatchCmd?.(`take ${capturedId}`));
          row.appendChild(btn);
        }
        groundEl.appendChild(row);
      }
    }
  }

  const exitsEl = document.getElementById('kids-exits');
  if (exitsEl) {
    while (exitsEl.firstChild) exitsEl.removeChild(exitsEl.firstChild);
    const DIR_ARROW: Record<string, string> = {
      north: '↑ North', south: '↓ South', east: '→ East', west: '← West',
      up: '▲ Up', down: '▼ Down',
    };
    for (const exit of (state.exits ?? [])) {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'kids-exit-btn';
      btn.textContent = DIR_ARROW[exit.toLowerCase()] ?? exit;
      const captured = exit;
      btn.addEventListener('click', () => _dispatchCmd?.(captured));
      exitsEl.appendChild(btn);
    }
  }
}

interface KidsActionDef {
  kidsAction: string;
  icon: string;
  label: string;
  cmd?: string;
  special?: string;
}

function rebuildKidsActionButtons(state: StateUpdate): void {
  const grid = document.getElementById('action-grid');
  if (!grid) return;
  while (grid.firstChild) grid.removeChild(grid.firstChild);

  const npcs      = state.room_npcs      ?? [];
  const resources = state.room_resources ?? [];

  const defs: KidsActionDef[] = [
    { kidsAction: 'look',   icon: '👁',  label: 'Look',   cmd: 'look' },
    ...(npcs.some(n => n.can_talk)   ? [{ kidsAction: 'talk',   icon: '💬', label: 'Talk' }]   : []),
    ...(npcs.some(n => n.attackable) ? [{ kidsAction: 'attack', icon: '⚔️', label: 'Attack' }] : []),
    ...(npcs.some(n => n.can_trade)  ? [{ kidsAction: 'trade',  icon: '🛒', label: 'Trade' }]  : []),
    ...(resources.length > 0         ? [{ kidsAction: 'forage', icon: '🌿', label: 'Forage' }] : []),
    { kidsAction: 'search', icon: '🔍', label: 'Search',  cmd: 'search' },
    { kidsAction: 'skills', icon: '⚡',  label: 'Skills',  cmd: 'skills' },
    { kidsAction: 'quests', icon: '📋', label: 'Quests',  special: 'quests-modal' },
    { kidsAction: 'craft',  icon: '🔧', label: 'Craft',   special: 'craft' },
  ];

  for (const a of defs) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'action-btn' + (a.special === 'craft' ? ' craft-btn' : '');
    btn.dataset.kidsAction = a.kidsAction;
    if (a.cmd)     btn.dataset.cmd     = a.cmd;
    if (a.special) btn.dataset.special = a.special;
    const icon = document.createElement('span');
    icon.className = 'btn-icon';
    icon.textContent = a.icon;
    btn.appendChild(icon);
    btn.appendChild(document.createTextNode(' ' + a.label));
    grid.appendChild(btn);
  }
}

function showTargetPicker(
  label: string,
  targets: Array<{ id: string; name: string }>,
  onPick: (id: string) => void,
): void {
  const picker      = document.getElementById('target-picker');
  const pickerLabel = document.getElementById('target-picker-label');
  const pickerBtns  = document.getElementById('target-picker-btns');
  if (!picker || !pickerLabel || !pickerBtns) return;

  pickerLabel.textContent = label;
  while (pickerBtns.firstChild) pickerBtns.removeChild(pickerBtns.firstChild);

  for (const t of targets) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'target-btn';
    btn.textContent = t.name;
    const capturedId = t.id;
    btn.addEventListener('click', () => {
      hideTargetPicker();
      onPick(capturedId);
    });
    pickerBtns.appendChild(btn);
  }
  picker.classList.add('open');
}

function hideTargetPicker(): void {
  document.getElementById('target-picker')?.classList.remove('open');
}

function openKidsQuestModal(): void {
  const modal = document.getElementById('quest-kids-modal');
  const list  = document.getElementById('quest-kids-list');
  if (!modal || !list) return;

  while (list.firstChild) list.removeChild(list.firstChild);

  const questData = _lastState?.quests ?? [];

  if (questData.length === 0) {
    const empty = document.createElement('div');
    empty.style.cssText = 'font-size:0.78rem;color:var(--comment);text-align:center;padding:1rem 0';
    empty.textContent = 'No active quests yet.';
    list.appendChild(empty);
  } else {
    for (const q of questData) {
      const card = document.createElement('div');
      card.className = 'quest-kids-card';

      const title = document.createElement('div');
      title.className = 'quest-kids-title';
      title.textContent = q.title;

      if (q.description) {
        const desc = document.createElement('div');
        desc.className = 'quest-kids-desc';
        desc.textContent = q.description;
        card.appendChild(title);
        card.appendChild(desc);
      } else {
        card.appendChild(title);
      }

      const barWrap = document.createElement('div');
      barWrap.className = 'quest-progress-bar-wrap';

      const barFill = document.createElement('div');
      barFill.className = 'quest-progress-bar-fill';
      const pct = q.obj_count > 0
        ? Math.min(100, Math.round((q.obj_progress / q.obj_count) * 100))
        : 0;
      barFill.style.width = `${pct}%`;
      barWrap.appendChild(barFill);

      const progressLabel = document.createElement('div');
      progressLabel.className = 'quest-progress-label';
      if (q.obj_count > 1) {
        progressLabel.textContent = `${q.obj_progress} of ${q.obj_count}`;
      } else if (q.obj_progress > 0) {
        progressLabel.textContent = 'Complete!';
      } else {
        progressLabel.textContent = 'In progress';
      }

      card.appendChild(barWrap);
      card.appendChild(progressLabel);
      list.appendChild(card);
    }
  }

  modal.classList.add('open');
}

const SKILL_MAX_LEVEL = 5;

function openKidsSkillsModal(): void {
  const modal = document.getElementById('skills-kids-modal');
  const list  = document.getElementById('skills-kids-list');
  if (!modal || !list) return;

  while (list.firstChild) list.removeChild(list.firstChild);

  const skillData = _lastState?.skills ?? [];

  const SKILL_EMOJI: Record<string, string> = {
    hacking:     '⚡',
    lockpicking: '🔓',
  };

  if (skillData.length === 0) {
    const empty = document.createElement('div');
    empty.style.cssText = 'font-size:0.78rem;color:var(--comment);text-align:center;padding:1rem 0';
    empty.textContent = "You haven't learned any skills yet. Try exploring the world!";
    list.appendChild(empty);
  } else {
    for (const s of skillData) {
      const card = document.createElement('div');
      card.className = 'skill-kids-card';

      const name = document.createElement('div');
      name.className = 'skill-kids-name';
      const emoji = SKILL_EMOJI[s.name] ?? '✨';
      name.textContent = `${emoji} ${s.name.charAt(0).toUpperCase() + s.name.slice(1)}`;

      const stars = document.createElement('div');
      stars.className = 'skill-kids-stars';
      stars.textContent = '★'.repeat(s.level) + '☆'.repeat(Math.max(0, SKILL_MAX_LEVEL - s.level));

      card.appendChild(name);
      card.appendChild(stars);
      list.appendChild(card);
    }
  }

  modal.classList.add('open');
}

// ── Player list ───────────────────────────────────────────────────────────────

let _myPlayerID = '';
let _worldName = 'cyberspace';
let _kidsMode = false;
let _lastState: StateUpdate | null = null;

/** Set the world name before calling initMUD. Called by game.astro. */
export function setWorld(name: string): void {
  _worldName = name;
}

let _teleportFn: ((targetID: string) => void) | null = null;
let _dispatchCmd: ((cmd: string) => void) | null = null;

function renderPlayerList(data: { hostOnline: boolean; players: Array<{ name: string; roomName?: string }> }) {
  const list = document.getElementById('player-list');
  if (!list) return;
  list.innerHTML = '';

  const hostRow = buildPlayerRow('gl1tch', 'host', data.hostOnline, '');
  list.appendChild(hostRow);

  for (const p of data.players) {
    const row = buildPlayerRow(p.name, 'peer', true, p.roomName ?? '');
    list.appendChild(row);
  }
}

function buildPlayerRow(name: string, role: 'host' | 'peer', online: boolean, roomName: string): HTMLElement {
  const row = document.createElement('div');
  row.className = 'player-row';

  const avatar = document.createElement('div');
  avatar.className = `player-avatar ${role}-av`;
  avatar.textContent = name.slice(0, 2);

  const info = document.createElement('div');
  info.className = 'player-info';

  const nameEl = document.createElement('div');
  nameEl.className = `player-name ${role}-name`;
  nameEl.textContent = name;

  const statusEl = document.createElement('div');
  statusEl.className = 'player-status';
  const loc = roomName ? roomName : (role === 'host' ? 'host' : 'online');
  statusEl.textContent = loc;

  info.appendChild(nameEl);
  info.appendChild(statusEl);

  const dot = document.createElement('div');
  dot.className = online
    ? (role === 'host' ? 'online-dot host-dot' : 'online-dot peer-dot')
    : 'online-dot offline-dot';

  row.appendChild(avatar);
  row.appendChild(info);
  row.appendChild(dot);

  // Teleport button — only for peers (not self, not host entry)
  if (role === 'peer' && name !== _myPlayerID && _teleportFn) {
    const gotoBtn = document.createElement('button');
    gotoBtn.className = 'goto-btn';
    gotoBtn.title = `Teleport to ${name}`;
    gotoBtn.textContent = '⤴';
    const captured = name;
    gotoBtn.addEventListener('click', () => _teleportFn!(captured));
    row.appendChild(gotoBtn);
  }

  return row;
}

// ── Chat ──────────────────────────────────────────────────────────────────────

const MAX_CHAT_LINES = 40;

function appendChatMessage(from: string, text: string, myID: string) {
  const container = document.getElementById('chat-messages');
  if (!container) return;

  const line = document.createElement('div');
  const isMe = from === myID;
  line.className = `chat-line ${isMe ? 'from-host' : 'from-peer'}`;

  const fromEl = document.createElement('span');
  fromEl.className = 'chat-from';
  fromEl.textContent = from + ': ';  // textContent is XSS-safe

  const textEl = document.createElement('span');
  textEl.className = 'chat-text';
  textEl.textContent = text;          // textContent is XSS-safe

  line.appendChild(fromEl);
  line.appendChild(textEl);
  container.appendChild(line);

  // Trim old messages
  while (container.children.length > MAX_CHAT_LINES) {
    container.removeChild(container.firstChild!);
  }

  container.scrollTop = container.scrollHeight;
}

// ── Craft modal + drag-and-drop ───────────────────────────────────────────────

// Client-side recipe store, refreshed on every state.update.
let _recipes: Recipe[] = [];

// Items currently placed in the 9 craft grid slots (null = empty).
const _craftSlots: (InvItem | null)[] = Array(9).fill(null);

// The recipe that currently matches the placed items, if any.
let _matchedRecipe: Recipe | null = null;

/** Count item IDs in the craft grid and match against known recipes. */
function matchRecipe(): Recipe | null {
  const counts: Record<string, number> = {};
  for (const item of _craftSlots) {
    if (item) counts[item.id] = (counts[item.id] ?? 0) + 1;
  }
  const placedKeys = Object.keys(counts);
  if (placedKeys.length === 0) return null;

  for (const recipe of _recipes) {
    const req: Record<string, number> = {};
    for (const ing of recipe.ingredients) {
      req[ing.id] = (req[ing.id] ?? 0) + ing.count;
    }
    const reqKeys = Object.keys(req);
    if (reqKeys.length !== placedKeys.length) continue;
    if (reqKeys.every(k => counts[k] === req[k]) && placedKeys.every(k => req[k] === counts[k])) {
      return recipe;
    }
  }
  return null;
}

/** Re-render craft grid slots and output after any change. */
function refreshCraftGrid() {
  const grid = document.getElementById('craft-grid');
  if (!grid) return;

  // Update each slot element.
  const slots = grid.querySelectorAll<HTMLElement>('.craft-slot');
  slots.forEach((el, i) => {
    const item = _craftSlots[i];
    el.innerHTML = '';
    el.classList.toggle('filled', !!item);
    if (item) {
      const icon = document.createElement('span');
      icon.className = 'craft-slot-icon';
      icon.textContent = TIER_ICON[item.tier] ?? '◦';
      const label = document.createElement('span');
      label.className = 'craft-slot-label';
      label.textContent = item.name.length > 6 ? item.name.slice(0, 5) + '…' : item.name;
      el.appendChild(icon);
      el.appendChild(label);
    } else {
      el.textContent = '+';
    }
  });

  // Update output slot and Craft button.
  _matchedRecipe = matchRecipe();
  const outputEl = document.getElementById('craft-output');
  const craftBtn = document.getElementById('craft-do-btn') as HTMLButtonElement | null;
  const hintEl   = document.getElementById('craft-hint');

  if (_matchedRecipe) {
    if (outputEl) outputEl.textContent = '⚙';
    if (craftBtn) {
      craftBtn.textContent = `🔧 Craft: ${_matchedRecipe.name}`;
      craftBtn.disabled = false;
    }
    if (hintEl) hintEl.textContent = `Recipe matched: ${_matchedRecipe.name}`;
  } else {
    if (outputEl) outputEl.textContent = '?';
    if (craftBtn) {
      const hasItems = _craftSlots.some(s => s !== null);
      craftBtn.textContent = hasItems ? '🔧 No matching recipe' : '🔧 Open Crafting Menu';
      craftBtn.disabled = hasItems; // disable when items placed but no match
    }
    if (hintEl) hintEl.textContent = hasItems() ? 'No matching recipe for these items.' : 'Drag items from your inventory into the grid.';
  }

  function hasItems() { return _craftSlots.some(s => s !== null); }
}

/** Wire drag-and-drop onto a craft slot element at index i. */
function wireCraftSlot(el: HTMLElement, i: number) {
  el.addEventListener('dragover', (e) => {
    e.preventDefault();
    el.classList.add('drag-over');
  });
  el.addEventListener('dragleave', () => {
    el.classList.remove('drag-over');
  });
  el.addEventListener('drop', (e) => {
    e.preventDefault();
    el.classList.remove('drag-over');
    const raw = e.dataTransfer?.getData('application/gl1tch-item');
    if (!raw) return;
    try {
      const item: InvItem = JSON.parse(raw);
      _craftSlots[i] = item;
      refreshCraftGrid();
    } catch { /* ignore bad data */ }
  });
  // Click a filled slot to clear it.
  el.addEventListener('click', () => {
    if (_craftSlots[i]) {
      _craftSlots[i] = null;
      refreshCraftGrid();
    }
  });
}

// ── Item modal ────────────────────────────────────────────────────────────────

let _currentItem: InvItem | null = null;
let _sendCmd: ((cmd: string) => void) | null = null;

function openItemModal(item: InvItem, sendCmd: (cmd: string) => void) {
  _currentItem = item;
  _sendCmd = sendCmd;

  const modal    = document.getElementById('item-modal')!;
  const iconEl   = document.getElementById('item-big-icon')!;
  const nameEl   = document.getElementById('item-name-display')!;
  const tierEl   = document.getElementById('item-tier-badge')!;
  const descEl   = document.getElementById('item-desc-display')!;

  iconEl.textContent = TIER_ICON[item.tier] ?? '◦';
  nameEl.textContent = item.name;
  descEl.textContent = item.desc || 'No description.';

  const tier = item.tier || 'noise';
  tierEl.textContent = tier;
  tierEl.className = `item-tier tier-badge-${tier}`;

  // Tint the icon border by tier
  const bigIcon = iconEl as HTMLElement;
  const tierColors: Record<string, string> = {
    'noise':    '#44475a',
    'signal':   '#2d6e7a',
    'ghost':    '#4a3070',
    'zero-day': '#6e4a1a',
    'flatline': '#6e1a1a',
  };
  bigIcon.style.borderColor = tierColors[tier] ?? '#44475a';

  modal.classList.add('open');
}

function closeItemModal() {
  document.getElementById('item-modal')!.classList.remove('open');
  _currentItem = null;
}

// ── Craft modal ───────────────────────────────────────────────────────────────

function openCraftModal() {
  // Reset slot state.
  _craftSlots.fill(null);
  _matchedRecipe = null;

  const grid = document.getElementById('craft-grid')!;
  grid.innerHTML = '';
  for (let i = 0; i < 9; i++) {
    const s = document.createElement('div');
    s.className = 'craft-slot';
    s.textContent = '+';
    wireCraftSlot(s, i);
    grid.appendChild(s);
  }
  refreshCraftGrid();
  document.getElementById('craft-modal')!.classList.add('open');
}

function closeCraftModal() {
  document.getElementById('craft-modal')!.classList.remove('open');
}

// ── Main init ─────────────────────────────────────────────────────────────────

/**
 * Builds the WebSocket URL for the given world.
 * Exported for testing.
 */
export function buildWsUrl(protocol: string, host: string, worldName: string): string {
  return `${protocol}//${host}/ws?world=${encodeURIComponent(worldName)}`;
}

export function initMUD() {
  const loginScreen = document.getElementById('login-screen')!;
  const hudScreen   = document.getElementById('hud-screen')!;
  const nameInput   = document.getElementById('player-name') as HTMLInputElement;
  const passInput   = document.getElementById('passphrase') as HTMLInputElement;
  const connectBtn  = document.getElementById('connect-btn') as HTMLButtonElement;
  const errorDiv    = document.getElementById('login-error')!;

  const outputEl  = document.getElementById('output')!;
  const cmdInput  = document.getElementById('cmd-input') as HTMLInputElement;
  const sendBtn   = document.getElementById('send-btn') as HTMLButtonElement;
  const roomEl    = document.getElementById('room-name')!;
  const hpHearts  = document.getElementById('hp-hearts')!;
  const hpText    = document.getElementById('hp-text')!;
  const creditsEl = document.getElementById('credits-display')!;

  let ws: WebSocket | null = null;
  let inputEnabled = false;

  // ── Chat input wiring ───────────────────────────────────────────────────────

  const chatInput  = document.getElementById('chat-input') as HTMLInputElement;
  const chatSendBtn = document.getElementById('chat-send-btn') as HTMLButtonElement;

  function sendChat() {
    const text = chatInput.value.trim();
    if (!text || !ws || ws.readyState !== WebSocket.OPEN) return;
    chatInput.value = '';
    ws.send(JSON.stringify({ type: 'chat', payload: { text } }));
  }

  chatSendBtn.addEventListener('click', sendChat);
  chatInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') sendChat(); });

  // ── Login ──────────────────────────────────────────────────────────────────

  connectBtn.addEventListener('click', connect);
  nameInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') connect(); });
  passInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') connect(); });

  const savedID   = localStorage.getItem('glitch-mud-player');
  const savedPass = localStorage.getItem('glitch-mud-pass');
  if (savedID) {
    nameInput.value = savedID;
    if (savedPass) passInput.value = savedPass;
    setTimeout(() => connect(), 50);
  }

  function showError(msg: string) {
    errorDiv.textContent = msg;
  }

  function connect() {
    const playerID   = nameInput.value.trim();
    const passphrase = passInput.value;

    if (!playerID) { showError('enter a handle to connect'); return; }
    if (!/^[a-zA-Z0-9-]{2,20}$/.test(playerID)) {
      showError('handle must be 2-20 alphanumeric characters or hyphens');
      return;
    }

    showError('');
    connectBtn.disabled = true;
    connectBtn.textContent = 'connecting...';

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(buildWsUrl(proto, location.host, _worldName));

    ws.onopen = () => {
      ws!.send(JSON.stringify({ type: 'auth', payload: { playerID, passphrase } }));
    };
    ws.onmessage = (evt) => {
      const msg: ServerMsg = JSON.parse(evt.data as string);
      handleServerMsg(msg);
    };
    ws.onerror = () => {
      connectBtn.disabled = false;
      connectBtn.textContent = 'connect';
      showError('connection failed — is the server running?');
      localStorage.removeItem('glitch-mud-player');
      localStorage.removeItem('glitch-mud-pass');
    };
    ws.onclose = () => {
      inputEnabled = false;
      appendOutput('\n\x1b[31m[disconnected from server]\x1b[0m\n');
      setInputEnabled(false);
    };
  }

  // ── Message handler ────────────────────────────────────────────────────────

  function handleServerMsg(msg: ServerMsg) {
    switch (msg.type) {
      case 'world_meta': {
        const meta = msg.payload as import('./theme').WorldMeta;
        import('./theme').then(({ applyTheme }) => {
          applyTheme(meta);
        });
        if (meta.ui_profile === 'kids') {
          applyKidsMode();
        }
        rebuildActionButtons(meta.name);
        break;
      }
      case 'auth.ok':
        _myPlayerID = nameInput.value.trim();
        _teleportFn = (targetID: string) => {
          if (inputEnabled) sendCommand(`goto ${targetID}`);
          else { cmdInput.value = `goto ${targetID}`; cmdInput.focus(); }
        };
        _dispatchCmd = (cmd: string) => { if (inputEnabled) sendCommand(cmd); };
        localStorage.setItem('glitch-mud-player', _myPlayerID);
        localStorage.setItem('glitch-mud-pass', passInput.value);
        showHUD();
        break;
      case 'auth.fail':
        connectBtn.disabled = false;
        connectBtn.textContent = 'connect';
        showError(msg.payload?.reason ?? 'authentication failed');
        localStorage.removeItem('glitch-mud-player');
        localStorage.removeItem('glitch-mud-pass');
        break;
      case 'output.token':
        appendOutput(msg.payload?.token ?? '');
        break;
      case 'output.done':
        setInputEnabled(true);
        scrollOutputToBottom();
        break;
      case 'state.update':
        applyStateUpdate(msg.payload as StateUpdate);
        break;
      case 'players.update':
        renderPlayerList(msg.payload as { hostOnline: boolean; players: Array<{ name: string; roomName?: string }> });
        break;
      case 'chat.message': {
        const { from, text } = msg.payload as { from: string; text: string };
        appendChatMessage(from, text, _myPlayerID);
        // Also echo into the main output so it's not missed while reading.
        appendOutput(`\n\x1b[36m[${escapeHtml(from)}]\x1b[0m ${escapeHtml(text)}\n`);
        break;
      }
      case 'error':
        appendOutput(`\n\x1b[31m[${msg.payload?.message ?? 'error'}]\x1b[0m\n`);
        break;
    }
  }

  // ── HUD helpers ──────────────────────────────────────────────────────────

  function rebuildActionButtons(worldName: string) {
    const grid = document.getElementById('action-grid');
    if (!grid) return;
    while (grid.firstChild) grid.removeChild(grid.firstChild);
    const actions = WORLD_ACTIONS[worldName] ?? DEFAULT_ACTIONS;
    for (const a of actions) {
      const btn = document.createElement('button');
      btn.className = 'action-btn' + (a.cls ? ' ' + a.cls : '');
      if (a.cmd)     btn.dataset.cmd     = a.cmd;
      if (a.special) btn.dataset.special = a.special;
      const icon = document.createElement('span');
      icon.className = 'btn-icon';
      icon.textContent = a.icon;
      btn.appendChild(icon);
      btn.appendChild(document.createTextNode(' ' + a.label));
      grid.appendChild(btn);
    }
  }

  // ── Kids onboarding hints ──────────────────────────────────────────────────
  const HINTS_KEY = 'bh_hints_seen';

  function getSeenHints(): Set<string> {
    try {
      const raw = localStorage.getItem(HINTS_KEY);
      return new Set(raw ? JSON.parse(raw) as string[] : []);
    } catch { return new Set(); }
  }

  function markHintSeen(key: string): void {
    const seen = getSeenHints();
    seen.add(key);
    localStorage.setItem(HINTS_KEY, JSON.stringify([...seen]));
  }

  function showHint(key: string, message: string): void {
    if (!_kidsMode) return;
    if (getSeenHints().has(key)) return;
    markHintSeen(key);

    const banner = document.getElementById('hint-banner');
    if (!banner) return;

    while (banner.firstChild) banner.removeChild(banner.firstChild);

    const text = document.createElement('span');
    text.textContent = message;

    const close = document.createElement('button');
    close.type = 'button';
    close.className = 'hint-close';
    close.textContent = '✕';
    close.addEventListener('click', () => banner.classList.remove('visible'));

    banner.appendChild(text);
    banner.appendChild(close);
    banner.classList.add('visible');

    setTimeout(() => banner.classList.remove('visible'), 5000);
  }

  function showHUD() {
    loginScreen.style.display = 'none';
    hudScreen.classList.add('active');
    setInputEnabled(true);
    if (_kidsMode) {
      setTimeout(() => showHint('first_login', "Click the buttons below to do things. They change based on what's around you!"), 1500);
    }
    cmdInput.focus();
    renderInventory([], (item) => openItemModal(item, sendCommand));
    updateCompass([]);
    rebuildActionButtons(_worldName);
  }

  function appendOutput(text: string) {
    const normalized = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const safeHtml   = ansiToHtml(normalized).replace(/\n/g, '<br>');
    const span = document.createElement('span');
    span.innerHTML = safeHtml;
    outputEl.appendChild(span);
    scrollOutputToBottom();
  }

  function scrollOutputToBottom() {
    outputEl.scrollTop = outputEl.scrollHeight;
  }

  function setInputEnabled(enabled: boolean) {
    inputEnabled      = enabled;
    cmdInput.disabled = !enabled;
    sendBtn.disabled  = !enabled;
    if (enabled) cmdInput.focus();
  }

  function applyStateUpdate(state: StateUpdate) {
    _lastState = state;
    if (_kidsMode) {
      if ((state.room_npcs ?? []).length > 0) {
        showHint('first_npc', "Someone's here! Click Talk to chat with them.");
      }
      if ((state.inventory ?? []).length > 0) {
        showHint('first_item', "Check Your Stuff to see what you're carrying.");
      }
      if ((state.quests ?? []).length > 0) {
        showHint('first_quest', 'A new quest! Tap Quests to track your progress.');
      }
    }
    roomEl.textContent    = state.roomName || '—';
    hpHearts.innerHTML    = renderHearts(state.hp, state.maxHp);
    hpText.textContent    = `${state.hp}/${state.maxHp}`;
    creditsEl.textContent = `¢ ${state.credits}`;
    if (state.recipes) _recipes = state.recipes;

    if (_kidsMode) {
      rebuildRoomContext(state);
      rebuildKidsActionButtons(state);
    } else {
      updateCompass(state.exits ?? []);
    }
    renderInventory(state.inventory ?? [], (item) => openItemModal(item, sendCommand));
  }

  // ── Input ─────────────────────────────────────────────────────────────────

  function sendCommand(cmd: string) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    cmd = cmd.trim();
    if (!cmd) return;
    appendOutput(`\n\x1b[32m>\x1b[0m ${cmd}\n`);
    setInputEnabled(false);
    ws.send(JSON.stringify({ type: 'input', payload: { text: cmd } }));
  }

  sendBtn.addEventListener('click', () => {
    if (!inputEnabled) return;
    const cmd = cmdInput.value;
    cmdInput.value = '';
    sendCommand(cmd);
  });

  cmdInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && inputEnabled) {
      const cmd = cmdInput.value;
      cmdInput.value = '';
      sendCommand(cmd);
    }
  });

  // ── Compass buttons ────────────────────────────────────────────────────────

  for (const [dir, id] of Object.entries(DIR_BUTTONS)) {
    const btn = document.getElementById(id);
    if (btn) {
      btn.addEventListener('click', () => {
        if (btn.classList.contains('active') && inputEnabled) {
          sendCommand(dir);
        }
      });
    }
  }

  const lookBtn = document.getElementById('btn-look');
  if (lookBtn) {
    lookBtn.addEventListener('click', () => {
      if (inputEnabled) sendCommand('look');
    });
  }

  // ── Action buttons (delegated) ─────────────────────────────────────────────

  function handleKidsAction(action: string): void {
    if (action === 'quests') {
      openKidsQuestModal();
      return;
    }
    if (action === 'skills') {
      openKidsSkillsModal();
      return;
    }
    if (!inputEnabled) return;
    const state = _lastState;

    if (action === 'look' || action === 'search') {
      hideTargetPicker();
      sendCommand(action);
      return;
    }
    if (action === 'craft') {
      return; // handled by the existing craft special in the click listener
    }

    if (action === 'talk') {
      const talkers = (state?.room_npcs ?? []).filter(n => n.can_talk);
      if (talkers.length === 0) return;
      if (talkers.length === 1) { sendCommand(`talk ${talkers[0].id}`); return; }
      showTargetPicker('Who do you want to talk to?', talkers, id => sendCommand(`talk ${id}`));
      return;
    }

    if (action === 'attack') {
      const hostiles = (state?.room_npcs ?? []).filter(n => n.attackable);
      if (hostiles.length === 0) return;
      if (hostiles.length === 1) { sendCommand(`attack ${hostiles[0].id}`); return; }
      showTargetPicker('Who do you want to attack?', hostiles, id => sendCommand(`attack ${id}`));
      return;
    }

    if (action === 'trade') {
      const traders = (state?.room_npcs ?? []).filter(n => n.can_trade);
      if (traders.length === 0) return;
      if (traders.length === 1) { sendCommand(`trade ${traders[0].id}`); return; }
      showTargetPicker('Who do you want to trade with?', traders, id => sendCommand(`trade ${id}`));
      return;
    }

    if (action === 'forage') {
      const resources = state?.room_resources ?? [];
      if (resources.length === 0) return;
      if (resources.length === 1) {
        sendCommand(`${resources[0].action} ${resources[0].id}`);
        return;
      }
      const namedResources = resources.map(r => ({ id: r.id, name: formatResourceName(r.id) }));
      showTargetPicker('What do you want to gather?', namedResources, id => {
        const res = resources.find(r => r.id === id);
        if (res) sendCommand(`${res.action} ${res.id}`);
      });
    }
  }

  document.getElementById('action-grid')?.addEventListener('click', (e) => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>('.action-btn');
    if (!btn) return;
    if (btn.dataset.special === 'craft') { openCraftModal(); return; }

    if (_kidsMode && btn.dataset.kidsAction) {
      handleKidsAction(btn.dataset.kidsAction);
      return;
    }

    if (btn.dataset.cmd && inputEnabled) sendCommand(btn.dataset.cmd);
  });

  document.addEventListener('click', (e) => {
    const picker = document.getElementById('target-picker');
    const grid   = document.getElementById('action-grid');
    if (
      picker &&
      grid &&
      !picker.contains(e.target as Node) &&
      !grid.contains(e.target as Node)
    ) {
      hideTargetPicker();
    }
  });

  // ── Craft modal ────────────────────────────────────────────────────────────

  document.getElementById('craft-modal-close')?.addEventListener('click', closeCraftModal);

  document.getElementById('craft-do-btn')?.addEventListener('click', () => {
    if (_matchedRecipe) {
      const recipeId = _matchedRecipe.id;
      closeCraftModal();
      if (inputEnabled) sendCommand(`craft ${recipeId}`);
      else { cmdInput.value = `craft ${recipeId}`; cmdInput.focus(); }
    } else {
      closeCraftModal();
      if (inputEnabled) sendCommand('craft');
      else { cmdInput.value = 'craft '; cmdInput.focus(); }
    }
  });

  // Close craft modal on overlay click
  document.getElementById('craft-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeCraftModal();
  });

  // ── Kids input toggle ──────────────────────────────────────────────────────
  const kidsToggle    = document.getElementById('kids-input-toggle');
  const INPUT_OPEN_KEY = 'bh-kids-input-open';

  function applyInputVisibility(open: boolean): void {
    const cmdEl    = document.getElementById('cmd-input');
    const sendEl   = document.getElementById('send-btn');
    const promptEl = document.querySelector<HTMLElement>('.prompt');
    if (open) {
      cmdEl?.classList.add('kids-visible');
      sendEl?.classList.add('kids-visible');
      promptEl?.classList.add('kids-visible');
      if (kidsToggle) kidsToggle.style.opacity = '1';
      (document.getElementById('cmd-input') as HTMLInputElement | null)?.focus();
    } else {
      cmdEl?.classList.remove('kids-visible');
      sendEl?.classList.remove('kids-visible');
      promptEl?.classList.remove('kids-visible');
      if (kidsToggle) kidsToggle.style.opacity = '0.45';
    }
  }

  if (kidsToggle) {
    const savedOpen = localStorage.getItem(INPUT_OPEN_KEY) === 'true';
    if (_kidsMode) applyInputVisibility(savedOpen);

    kidsToggle.addEventListener('click', () => {
      if (!_kidsMode) return;
      const isOpen = document.getElementById('cmd-input')?.classList.contains('kids-visible') ?? false;
      const next = !isOpen;
      applyInputVisibility(next);
      localStorage.setItem(INPUT_OPEN_KEY, String(next));
      if (next) showHint('first_type', 'You can also type commands if you want.');
    });
  }

  // ── Item modal ─────────────────────────────────────────────────────────────

  document.getElementById('item-modal-close')?.addEventListener('click', closeItemModal);

  document.getElementById('item-examine-btn')?.addEventListener('click', () => {
    if (!_currentItem) return;
    closeItemModal();
    if (inputEnabled) sendCommand(`examine ${_currentItem.id}`);
  });

  document.getElementById('item-use-btn')?.addEventListener('click', () => {
    if (!_currentItem) return;
    closeItemModal();
    if (inputEnabled) sendCommand(`use ${_currentItem.id}`);
    else {
      cmdInput.value = `use ${_currentItem.id}`;
      cmdInput.focus();
    }
  });

  document.getElementById('item-drop-btn')?.addEventListener('click', () => {
    if (!_currentItem) return;
    closeItemModal();
    if (inputEnabled) sendCommand(`drop ${_currentItem.id}`);
    else {
      cmdInput.value = `drop ${_currentItem.id}`;
      cmdInput.focus();
    }
  });

  // Close item modal on overlay click
  document.getElementById('item-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeItemModal();
  });

  document.getElementById('quest-kids-modal-close')?.addEventListener('click', () => {
    document.getElementById('quest-kids-modal')?.classList.remove('open');
  });

  document.getElementById('quest-kids-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) {
      (e.currentTarget as HTMLElement).classList.remove('open');
    }
  });

  document.getElementById('skills-kids-modal-close')?.addEventListener('click', () => {
    document.getElementById('skills-kids-modal')?.classList.remove('open');
  });

  document.getElementById('skills-kids-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) {
      (e.currentTarget as HTMLElement).classList.remove('open');
    }
  });
}
