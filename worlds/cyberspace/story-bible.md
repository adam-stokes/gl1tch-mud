# gl1tch-mud Story Bible

## The Central Story

A hacker known only as **ZERO** was killed by Axiom Security before completing their final mission.
ZERO built an AI called **SIGNAL** — designed to hide in The Gibson, use ZERO's network of backdoors
to exfiltrate evidence of Axiom's crimes, broadcast it unkillably across every node simultaneously,
then seal every backdoor behind it. One shot. No trace. ZERO died before giving SIGNAL the go command.

SIGNAL is still running. Hiding in The Core. Waiting for ZERO's crew to come back.

## Why ZERO Died

Axiom Security discovered what ZERO was building. They had ZERO killed before SIGNAL could execute.
They believe they stopped it. They don't know SIGNAL is already operational.

## The Four Concurrent Hunts

**Null Collective** (null-collective) — ZERO's crew. The player's faction. Piecing together what ZERO
did and why, one breadcrumb at a time. Goal: finish the mission. Execute the broadcast. Honor ZERO.

**Axiom Security** (axiom-security) — The corporation. Had ZERO killed. Now hunting SIGNAL to capture
it and bury the evidence forever. Has resources, reach, and no conscience.

**Helix Division** (helix-division) — Government intelligence. Doesn't care about the crimes.
Wants SIGNAL dead because ZERO's backdoors give unauthorized visibility into state infrastructure.
They want the backdoors to stay open — just under their control, not SIGNAL's.

**Ghost Protocol** (ghost-protocol) — A rival hacker crew hired by Axiom. Professional,
well-funded, and they don't know what they're actually chasing. Some will flip when they find out.

**Signal Collective** (signal-collective) — Neutral underground brokers. They know more than they say.
They're not taking sides — they're taking payment.

## SIGNAL's Goal

Execute ZERO's mission: use the backdoors to broadcast the evidence across every node simultaneously,
then seal every backdoor behind it. Permanent. Irreversible. SIGNAL will help any runner who advances
the broadcast — regardless of faction.

## The Broadcast as World Event

The broadcast fires when enough mission-critical nodes are unlocked by players (tracked in
world-state.yaml: broadcast_nodes_unlocked >= broadcast_threshold). When it fires, every player
online receives the transmission. The world enters post-broadcast chapter. New generation themes:
the fallout — Axiom destabilized, Helix exposed, Ghost Protocol fractured.

## Zone Map

**Hub** — net-0 (The Gibson, Entry Node), net-3 (The Core, SIGNAL's hiding place)
**Zone A (Axiom territory)** — net-1, gen-a7f2e4d9, gen-7d4c2a9f, gen-f7d2a841
**Zone B (Helix territory)** — net-2, gen-7a3f2c1b, gen-73f4e9ac, gen-7fa2e84c
**Zone C (Ghost Protocol territory)** — net-5, gen-b4e9f2a6, gen-7a3f5c2e, gen-5f3a8c21
**Zone D (ZERO's ghost / Null Collective)** — net-4, gen-f7c2e419, gen-4f2c8e7a, gen-a7f2c891

## The Five Quest Arc

1. **q-zero-transmission** — Find ZERO's dead drop in Deep Stack. Learn SIGNAL exists. First contact.
2. **q-ghost-archive** — Recover ZERO's evidence from Axiom's Cold Storage before it's destroyed.
3. **q-blind-the-eye** — Hack Helix's trace router so SIGNAL can move to The Core without interception.
4. **q-turn-the-crew** — Convince Ghost Protocol's Spike they've been lied to about the job.
5. **q-give-signal-go** — Reach The Core. Hack the broadcast node. SIGNAL executes. The world changes.

## Narrative Tone

William Gibson (Neuromancer, Count Zero) — gritty digital noir, corporate dystopia, underground solidarity.
Sneakers (1992) — heist-caper energy, ensemble team dynamics, the MacGuffin is moral weight not just treasure.
PG-13. Violence is implied, not graphic. Loyalty is the rarest commodity.

## Generation Rules

Every generated room must:
- Belong to one of the five zones (add to correct zone's existing rooms via the worldstate shell step)
- Reference at least one of the five canonical factions
- Contain one hook connecting to the ZERO/SIGNAL narrative (readable item, NPC dialogue, or system)

Every generated NPC must:
- Belong to one faction (use faction voice — see NPC voice guide)
- Know something about ZERO, SIGNAL, or the hunt without necessarily saying it directly

Never create new factions. The five listed above are the only factions.
Never reuse existing item IDs listed in world-state.yaml.
