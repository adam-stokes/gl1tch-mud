# Blockhaven — Story Bible

## Canonical Premise

The Crystal Core floated above Meadow Town Square, powering the five biomes of Blockhaven
with warmth, colour, and life. A young dragon named **Cinder** was playing in the sky and
accidentally collided with the Core, shattering it into five Crystal Shards that scattered
to each biome. Cinder, ashamed and frightened, fled to a hidden cave in the Snow Peaks.

Without the Core, the biomes are slowly fading.

---

## The World

### Five Biomes

| Biome | Faction | Shard | Crisis |
|---|---|---|---|
| Meadow | The Stoneguard | Meadow Shard | Flowers wilting, pedestal empty |
| Forest | The Thornwalkers | Forest Shard | Ancient trees going dormant |
| Desert | The Dunekeepers | Desert Shard | Ruins sinking faster into sand |
| Snow Peaks | The Frostborn | Mountain Shard | Blizzards worsening, Cinder's presence |
| Deep Caves | The Deepborn | Cave Shard | Crystal Flame extinguished, deep dark spreading |

### Five Factions

All five factions want the same thing: restore the Crystal Core. They have different
personalities and skills, but NO faction is an enemy. Rivalry is friendly at worst.

**The Stoneguard** — Master builders and craftspeople. Elder Mason leads with measured wisdom.
Tone: warm, practical, quietly worried.

**The Thornwalkers** — Forest rangers and herbalists. Warden Sylara leads. They commune with
nature and are deeply distressed by the dormant trees.
Tone: calm, observant, earthy.

**The Dunekeepers** — Desert archaeologists and puzzle-lovers. Archivist Dunes leads. They
see the Core's shattering as both catastrophe and fascinating mystery.
Tone: curious, academic, dry humor.

**The Frostborn** — Mountain miners and smiths. Ironmaster Breck leads. They express care
through competence — they'll forge whatever tool you need.
Tone: blunt, direct, trustworthy.

**The Deepborn** — Ancient cave dwellers, lore keepers of the Crystal Core's original
construction. Elder Voss leads. They speak slowly and in few words.
Tone: ancient, wise, patient.

---

## Cinder

Cinder is a young dragon — copper-and-gold scales, about the size of a large horse. She is:
- Not a villain. She is a child who made a terrible mistake.
- Ashamed and frightened, not aggressive.
- Intelligent and well-meaning.
- The ONLY entity with the fire to fuse the Crystal Shards back together.

When the player brings all five shards to Cinder, she fuses them with dragon breath and
together they restore the Crystal Core. This is cooperation, not defeat.

---

## Narrative Chapters

| Chapter ID | Condition | World State |
|---|---|---|
| `shard-hunt` | Default start | Crystal Core pedestal empty, biomes fading |
| `ember-found` | All 5 shards in inventory + q-cinder-return accepted | snow-0 northern passage unlocked |
| `core-restored` | q-cinder-return completed | Core restored, biomes revive (flavor text changes) |

---

## Generation Rules for Pipelines

When generating new rooms, ALWAYS:
- Assign a biome from: meadow, forest, desert, snow, caves
- Reference the faction that controls that biome
- Include one hint connecting to the Crystal Core narrative
- Use the tone of the biome's faction (see faction tones above)
- Include at least one mineable or harvestable resource per room
- Room IDs: gen-<8 lowercase hex chars>
- Item IDs: kebab-case-descriptive-name
- No new factions. No new dragons. No new magical cores.
- Cinder is the ONLY dragon in Blockhaven.
- Combat mobs should fit the biome (see mob roster in world.yaml)

### Vocabulary by Biome

**Meadow**: cobblestone, flowers, sun, open fields, building sites, workshops, cheerful
**Forest**: ancient trees, roots, moss, filtered light, quiet, watchful, earthy
**Desert**: sand, ruins, heat, brass, spyglass, archaeology, buried, ancient
**Snow Peaks**: ice, forge, stone, cold wind, hammer, strength, endurance
**Deep Caves**: crystal, glowstone, dark, echo, ancient, quiet, deep, patient

---

## What NOT to Do

- Do not create new factions
- Do not make any faction an enemy of another
- Do not introduce a new "villain" — there is no villain in Blockhaven
- Do not add new biomes
- Do not make Cinder hostile
- Do not add gory or violent descriptions — this world is for ages 8-12
- Do not use the cyberspace world's vocabulary (no "jacking in", "ICE", "hacking" as flavor)
  Note: the hack mechanic can exist (terminals, puzzle locks) but should be described as
  "solving the ancient puzzle" not "hacking"
