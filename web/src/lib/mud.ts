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
// All user-supplied or server-supplied text is HTML-entity-escaped BEFORE any
// span tags are inserted, so the resulting HTML is safe to set via innerHTML.

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

/** Escape HTML entities so raw text cannot contain tags. */
function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/**
 * Convert ANSI colour escape sequences to safe HTML spans.
 * The input is split on escape sequences; plain-text segments are
 * HTML-escaped before being concatenated, so no XSS is possible.
 */
function ansiToHtml(text: string): string {
  // Split on ANSI SGR sequences (\x1b[...m)
  const parts = text.split(/(\x1b\[[0-9;]*m)/);
  let openCount = 0;
  let out = '';

  for (const part of parts) {
    const m = part.match(/^\x1b\[([0-9;]*)m$/);
    if (m) {
      // It is a control sequence — never escape, just translate.
      const codes = m[1].split(';');
      for (const code of codes) {
        if (code === '0' || code === '') {
          // Reset — close all open spans.
          out += '</span>'.repeat(openCount);
          openCount = 0;
        } else if (ANSI_OPEN[code]) {
          out += `<span style="${ANSI_OPEN[code]}">`;
          openCount++;
        }
        // Unknown codes are silently dropped.
      }
    } else {
      // Plain text segment — escape entities.
      out += escapeHtml(part);
    }
  }

  // Close any unclosed spans.
  out += '</span>'.repeat(openCount);
  return out;
}

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

function renderInventory(items: InvItem[], sendCmd: (cmd: string) => void) {
  const grid = document.getElementById('inv-grid');
  if (!grid) return;

  // Remove existing slots.
  while (grid.firstChild) grid.removeChild(grid.firstChild);

  for (let i = 0; i < SLOTS; i++) {
    const slot = document.createElement('div');
    slot.className = 'inv-slot';

    if (i < items.length) {
      const item = items[i];
      slot.classList.add('occupied');
      slot.dataset.tier = item.tier || 'noise';
      const label = item.name.length > 12 ? item.name.slice(0, 11) + '\u2026' : item.name;
      slot.textContent = label; // textContent is XSS-safe
      slot.title = `${item.name}\n${item.desc}`;

      // Capture item.id in closure.
      const itemId = item.id;
      slot.addEventListener('click', () => sendCmd(`examine ${itemId}`));
    } else {
      slot.style.opacity = '0.3';
    }

    grid.appendChild(slot);
  }
}

// ── Compass ───────────────────────────────────────────────────────────────────

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

// ── Main init ─────────────────────────────────────────────────────────────────

export function initMUD() {
  // Login elements
  const loginScreen = document.getElementById('login-screen')!;
  const hudScreen   = document.getElementById('hud-screen')!;
  const nameInput   = document.getElementById('player-name') as HTMLInputElement;
  const passInput   = document.getElementById('passphrase') as HTMLInputElement;
  const connectBtn  = document.getElementById('connect-btn') as HTMLButtonElement;
  const errorDiv    = document.getElementById('login-error')!;

  // HUD elements
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

  function showError(msg: string) {
    errorDiv.textContent = msg; // textContent is safe
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
        showHUD();
        break;
      case 'auth.fail':
        connectBtn.disabled = false;
        connectBtn.textContent = 'connect';
        showError(msg.payload?.reason ?? 'authentication failed');
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
      case 'error':
        appendOutput(`\n\x1b[31m[${msg.payload?.message ?? 'error'}]\x1b[0m\n`);
        break;
    }
  }

  // ── HUD helpers ───────────────────────────────────────────────────────────

  function showHUD() {
    loginScreen.style.display = 'none';
    hudScreen.classList.add('active');
    setInputEnabled(true);
    cmdInput.focus();
    renderInventory([], sendCommand);
    updateCompass([]);
  }

  /**
   * Append game text to the output panel.
   * The text is ANSI-converted to safe HTML, then a <span> is appended.
   * Only our own ansiToHtml output (which escapes all plain text) is set
   * as innerHTML — no raw user or server strings are ever set directly.
   */
  function appendOutput(text: string) {
    const normalized = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const safeHtml   = ansiToHtml(normalized).replace(/\n/g, '<br>');

    const span = document.createElement('span');
    // safeHtml is produced entirely by ansiToHtml which HTML-escapes all
    // plain-text segments before inserting span tags — safe to use here.
    span.innerHTML = safeHtml;
    outputEl.appendChild(span);
    scrollOutputToBottom();
  }

  function scrollOutputToBottom() {
    outputEl.scrollTop = outputEl.scrollHeight;
  }

  function setInputEnabled(enabled: boolean) {
    inputEnabled     = enabled;
    cmdInput.disabled = !enabled;
    sendBtn.disabled  = !enabled;
    if (enabled) cmdInput.focus();
  }

  function applyStateUpdate(state: StateUpdate) {
    roomEl.textContent    = state.roomName || '\u2014';
    hpHearts.innerHTML    = renderHearts(state.hp, state.maxHp); // only colour codes, safe
    hpText.textContent    = `${state.hp}/${state.maxHp}`;
    creditsEl.textContent = `\u00a2 ${state.credits}`;
    updateCompass(state.exits ?? []);
    renderInventory(state.inventory ?? [], sendCommand);
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
}
