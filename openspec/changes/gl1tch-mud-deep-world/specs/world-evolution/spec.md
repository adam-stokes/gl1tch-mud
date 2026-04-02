## ADDED Requirements

### Requirement: explore command
`explore <direction>` SHALL:
1. Check whether the current room has an exit in that direction
2. If an exit exists, behave identically to `go <direction>`
3. If no exit exists, attempt Ollama-driven generation (see below)
4. If Ollama is unreachable or returns unparseable output, print "nothing there" and return without error

### Requirement: Ollama room generation
When `explore` finds no exit, gl1tch-mud SHALL:
1. Hash the prompt (current room ID + direction + world name) as SHA256
2. Check `generated_content` table for a cached entry with that hash; if found, load from cache
3. Otherwise, POST to `http://localhost:11434/api/chat` with model from `world.narrator_model` (default `llama3.2`)
4. Prompt SHALL include: world theme, current room name and description, direction, instruction to return a YAML room fragment
5. Parse the response as a `Room` struct; on parse failure, return "nothing there"
6. Assign the generated room a unique ID (`gen-<hash[:8]>`)
7. Add a bidirectional exit: current room → generated room (in `direction`), generated room → current room (opposite direction)
8. Persist room YAML to `generated_content` with the prompt hash and model name
9. Print the new room's description as if the player moved into it
10. Emit `mud.world.generated` with room ID, direction, and model used

#### Scenario: First explore in unmapped direction generates a room
- **WHEN** player types `explore west` and no west exit exists
- **THEN** Ollama is called, room parsed and persisted, player sees new room description

#### Scenario: Second explore in same direction loads from cache
- **WHEN** player types `explore west` again (after the first)
- **THEN** cached room is loaded from SQLite, no Ollama call

#### Scenario: Ollama unreachable
- **WHEN** Ollama is not running or times out (5s timeout)
- **THEN** command prints "static beyond the edge" and returns cleanly

#### Scenario: Ollama returns malformed YAML
- **WHEN** Ollama response cannot be parsed as a Room struct
- **THEN** command prints "something's there but you can't make it out" and returns

### Requirement: Generated room YAML prompt template
The prompt SHALL instruct the model to return ONLY a YAML block with fields: `name`, `desc`, `exits` (only the return direction), optionally one NPC and one item. No prose outside the YAML block.
