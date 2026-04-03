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
}

interface InvItem {
  id: string;
  name: string;
  desc: string;
  tier: string;
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

      const icon = document.createElement('div');
      icon.className = 'slot-icon';
      icon.textContent = TIER_ICON[item.tier] ?? '◦';

      const label = document.createElement('div');
      label.className = 'slot-label';
      const short = item.name.length > 10 ? item.name.slice(0, 9) + '…' : item.name;
      label.textContent = short;

      slot.appendChild(icon);
      slot.appendChild(label);
      slot.title = `${item.name}\n${item.desc}`;

      const captured = item;
      slot.addEventListener('click', () => onItemClick(captured));
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

// ── Player list ───────────────────────────────────────────────────────────────

function renderPlayerList(data: { hostOnline: boolean; players: Array<{ name: string }> }) {
  const list = document.getElementById('player-list');
  if (!list) return;
  list.innerHTML = '';

  // Host row
  const hostRow = buildPlayerRow('gl1tch', 'host', data.hostOnline);
  list.appendChild(hostRow);

  for (const p of data.players) {
    const row = buildPlayerRow(p.name, 'peer', true);
    list.appendChild(row);
  }
}

function buildPlayerRow(name: string, role: 'host' | 'peer', online: boolean): HTMLElement {
  const row = document.createElement('div');
  row.className = 'player-row';

  const avatar = document.createElement('div');
  avatar.className = `player-avatar ${role}-av`;
  avatar.textContent = name.slice(0, 2); // initials

  const info = document.createElement('div');
  info.className = 'player-info';

  const nameEl = document.createElement('div');
  nameEl.className = `player-name ${role}-name`;
  nameEl.textContent = name;

  const statusEl = document.createElement('div');
  statusEl.className = 'player-status';
  statusEl.textContent = role === 'host' ? 'host · online' : (online ? 'peer · online' : 'peer · offline');

  info.appendChild(nameEl);
  info.appendChild(statusEl);

  const dot = document.createElement('div');
  dot.className = online
    ? (role === 'host' ? 'online-dot host-dot' : 'online-dot peer-dot')
    : 'online-dot offline-dot';

  row.appendChild(avatar);
  row.appendChild(info);
  row.appendChild(dot);
  return row;
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
  // Build 9 empty craft slots
  const grid = document.getElementById('craft-grid')!;
  grid.innerHTML = '';
  for (let i = 0; i < 9; i++) {
    const s = document.createElement('div');
    s.className = 'craft-slot';
    s.textContent = '+';
    grid.appendChild(s);
  }
  document.getElementById('craft-modal')!.classList.add('open');
}

function closeCraftModal() {
  document.getElementById('craft-modal')!.classList.remove('open');
}

// ── Main init ─────────────────────────────────────────────────────────────────

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
    ws = new WebSocket(`${proto}//${location.host}/ws`);

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
      case 'auth.ok':
        localStorage.setItem('glitch-mud-player', nameInput.value.trim());
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
        renderPlayerList(msg.payload as { hostOnline: boolean; players: Array<{ name: string }> });
        break;
      case 'error':
        appendOutput(`\n\x1b[31m[${msg.payload?.message ?? 'error'}]\x1b[0m\n`);
        break;
    }
  }

  // ── HUD helpers ──────────────────────────────────────────────────────────

  function showHUD() {
    loginScreen.style.display = 'none';
    hudScreen.classList.add('active');
    setInputEnabled(true);
    cmdInput.focus();
    renderInventory([], (item) => openItemModal(item, sendCommand));
    updateCompass([]);
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
    roomEl.textContent    = state.roomName || '—';
    hpHearts.innerHTML    = renderHearts(state.hp, state.maxHp);
    hpText.textContent    = `${state.hp}/${state.maxHp}`;
    creditsEl.textContent = `¢ ${state.credits}`;
    updateCompass(state.exits ?? []);
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

  // ── Action buttons ─────────────────────────────────────────────────────────

  document.querySelectorAll<HTMLButtonElement>('.action-btn[data-cmd]').forEach((btn) => {
    btn.addEventListener('click', () => {
      if (inputEnabled) sendCommand(btn.dataset.cmd!);
    });
  });

  // ── Craft modal ────────────────────────────────────────────────────────────

  document.getElementById('open-craft-btn')?.addEventListener('click', () => {
    openCraftModal();
  });

  document.getElementById('craft-modal-close')?.addEventListener('click', closeCraftModal);

  document.getElementById('craft-do-btn')?.addEventListener('click', () => {
    closeCraftModal();
    if (inputEnabled) sendCommand('craft');
    else {
      // Drop into the terminal with the word pre-filled
      cmdInput.value = 'craft ';
      cmdInput.focus();
    }
  });

  // Close craft modal on overlay click
  document.getElementById('craft-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) closeCraftModal();
  });

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
}
