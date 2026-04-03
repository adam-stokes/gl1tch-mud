## ADDED Requirements

### Requirement: Login screen before terminal
On page load, the client SHALL display a login form with a player name field and passphrase field. The terminal SHALL NOT be visible until authentication succeeds.

#### Scenario: Page load shows login
- **WHEN** user navigates to the root URL
- **THEN** a login form is displayed with name and passphrase inputs and a Connect button; the terminal is hidden

#### Scenario: Successful login shows terminal
- **WHEN** user submits valid credentials and the server responds with `auth.ok`
- **THEN** the login form is hidden and the xterm.js terminal fills the viewport

#### Scenario: Failed login shows error
- **WHEN** server responds with `auth.fail`
- **THEN** an error message is displayed inline on the login form; the terminal remains hidden

### Requirement: Full-viewport xterm.js terminal
After authentication, the terminal SHALL occupy the full browser viewport with no visible scrollbars on the page itself. The terminal's own scrollback buffer SHALL handle overflow.

#### Scenario: Terminal fills viewport
- **WHEN** the terminal is shown
- **THEN** it fills 100% of viewport width and height with no page-level scroll

#### Scenario: Window resize
- **WHEN** the browser window is resized
- **THEN** the xterm.js instance resizes to match and a resize message is sent to the server (cols/rows)

### Requirement: Line-buffered input
The client SHALL buffer keystrokes locally in xterm.js and send the complete line to the server as an `input` message when the user presses Enter.

#### Scenario: Enter submits line
- **WHEN** user types `look` and presses Enter
- **THEN** client sends `{"type":"input","payload":{"text":"look"}}` and displays a newline in the terminal

#### Scenario: Backspace edits locally
- **WHEN** user presses Backspace
- **THEN** the local buffer is updated and the terminal reflects the deletion without a server round-trip

### Requirement: Server output written to terminal
Each `output.token` message from the server SHALL be written directly to the xterm.js terminal. ANSI escape codes in the token SHALL be rendered by xterm.js.

#### Scenario: Token rendered with ANSI
- **WHEN** server sends `{"type":"output.token","payload":{"token":"\x1b[32mACCESS GRANTED\x1b[0m\n"}}`
- **THEN** terminal displays "ACCESS GRANTED" in green

#### Scenario: output.done shows prompt
- **WHEN** server sends `output.done`
- **THEN** the terminal writes a `> ` prompt and re-enables local input

### Requirement: Ctrl-C sends interrupt
When the user presses Ctrl-C in the terminal, the client SHALL send `{"type":"interrupt"}` to the server.

#### Scenario: Ctrl-C during command
- **WHEN** user presses Ctrl-C while awaiting a response
- **THEN** client sends `{"type":"interrupt"}` and the terminal displays `^C\n`

### Requirement: Dark Dracula theme
The terminal and login page SHALL use the Dracula color palette. Background `#282a36`, foreground `#f8f8f2`, accent colors matching the gl1tch aesthetic.

#### Scenario: Terminal theme applied
- **WHEN** the xterm.js terminal is initialized
- **THEN** it uses Dracula background and foreground colors with matching cursor color
