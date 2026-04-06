// Package classes defines the mudout character classes: their starting kits,
// signature verbs, skill bonuses, and faction-rep deltas applied at creation.
package classes

// KitItem is one entry in a class's starting kit list.
type KitItem struct {
	ID   string // inventory item ID
	Name string // display name
	Desc string // inventory description
	Cost int    // kit-point cost (0 = free hook item)
}

// SkillBonus grants a starting skill level boost at character creation.
type SkillBonus struct {
	Skill string
	Level int
}

// RepDelta is a starting faction reputation adjustment.
type RepDelta struct {
	Faction string
	Delta   int
}

// Class is a complete class definition.
type Class struct {
	ID            string       // canonical lowercase id ("gunslinger", "ghoul", etc.)
	Name          string       // display name
	Tagline       string       // one-line flavor
	Flavor        string       // paragraph shown during creation
	MentorNPCID   string       // NPC id of the mentor in wakeup-camp
	SignatureVerb string       // command verb (e.g. "fan", "hotwire")
	GatingSkill   string       // skill that unlocks the verb for other classes at level 3
	SkillBonuses  []SkillBonus // starting skill levels
	StartingRep   []RepDelta   // starting reputation deltas
	Kit           []KitItem    // available kit items (player picks <= 10 cost)
}

// KitBudget is the total kit points each class gets to spend at creation.
const KitBudget = 10

// SignatureVerbUnlockLevel is the gating-skill level at which any class
// can use any class's signature verb.
const SignatureVerbUnlockLevel = 3

// All returns the registry of every class in load order.
func All() []Class {
	return []Class{gunslinger, mechanic, scavver, medic, ghoul}
}

// ByID returns the class with the given id, or nil if not found.
func ByID(id string) *Class {
	for i := range registryByID {
		if registryByID[i].ID == id {
			return &registryByID[i]
		}
	}
	return nil
}

var registryByID = All()

// ─── Class definitions ───────────────────────────────────────────────────

var gunslinger = Class{
	ID:            "gunslinger",
	Name:          "Gunslinger",
	Tagline:       "Old-world iron in a chrome world.",
	Flavor:        "You came up on stories of the Drift — lone shooters with nothing but a code and a six-gun. The wasteland chewed up the romance, but the iron still works.",
	MentorNPCID:   "old-cass",
	SignatureVerb: "fan",
	GatingSkill:   "combat",
	SkillBonuses:  []SkillBonus{{"combat", 1}},
	Kit: []KitItem{
		{"worn-revolver", "Worn Revolver", "Six-shot, walnut grip, fired enough to know it.", 3},
		{"lever-rifle", "Lever Rifle", "Long iron, slow but mean.", 5},
		{"speedloader-pair", "Speedloader x2", "Cuts your reload in half.", 1},
		{"patched-duster", "Patched Duster", "Heavy canvas. +2 defense.", 2},
		{"tin-star", "Tin Star Badge", "Bluffs Settler NPCs.", 1},
		{"whiskey-flask", "Whiskey Flask", "Heal 10 HP, 3 charges.", 1},
		{"bone-knife", "Bone-handle Knife", "1d4 melee.", 2},
		{"trail-rations", "Trail Rations", "Heal 5 HP, 5 charges.", 1},
		{"lucky-bullet", "Lucky Bullet", "One guaranteed crit.", 4},
		{"faded-photograph", "Faded Photograph", "A woman, a porch, a long time ago.", 0},
	},
}

var mechanic = Class{
	ID:            "mechanic",
	Name:          "Mechanic",
	Tagline:       "Keeps dead tech alive.",
	Flavor:        "Other people see junk. You see a power coupling, a working actuator, a generator that just needs a kick. The wasteland is full of dead things waiting to be coaxed back.",
	MentorNPCID:   "rust",
	SignatureVerb: "hotwire",
	GatingSkill:   "crafting",
	SkillBonuses:  []SkillBonus{{"crafting", 1}},
	Kit: []KitItem{
		{"pipe-pistol", "Pipe Pistol", "1d6, sidearm.", 3},
		{"wrench", "Wrench", "1d6 melee, doubles as crafting tool.", 2},
		{"welding-mask", "Welding Mask", "+1 defense, immune to flash.", 2},
		{"toolkit", "Toolkit", "Required for hotwiring advanced systems.", 3},
		{"scrap-metal-5", "Scrap Metal x5", "Crafting reagent.", 1},
		{"pipe-parts-3", "Pipe Parts x3", "Crafting reagent.", 1},
		{"spare-battery", "Spare Battery", "Powers hotwire once without parts.", 2},
		{"dust-goggles", "Dust Goggles", "+1 perception in dust storms.", 1},
		{"jury-rig-kit", "Jury-Rig Kit", "One field repair on a broken weapon.", 4},
		{"burnt-service-medal", "Burnt Service Medal", "Brass, soot-blackened, no name.", 0},
	},
}

var scavver = Class{
	ID:            "scavver",
	Name:          "Scavver",
	Tagline:       "Gets things. Doesn't ask how.",
	Flavor:        "Barter, lift, find. There's no lock you can't talk past, no crowd you can't disappear into, no junk pile that hasn't got something worth flipping.",
	MentorNPCID:   "halftrack",
	SignatureVerb: "barter",
	GatingSkill:   "trading",
	SkillBonuses:  []SkillBonus{{"scavenging", 1}, {"trading", 1}},
	Kit: []KitItem{
		{"sawed-off", "Sawed-Off Shotgun", "1d8 point-blank.", 4},
		{"switchblade", "Switchblade", "1d4 melee, +1 to lift.", 2},
		{"lockpicks-3", "Lockpicks x3", "For physical locks.", 1},
		{"fake-creds", "Fake Creds", "Bluffs Ironclad NPCs once.", 2},
		{"lucky-charm", "Lucky Charm", "+1 to first lift per day.", 2},
		{"big-satchel", "Big Satchel", "+5 inventory slots.", 1},
		{"rad-x-2", "Rad-X x2", "Temporary rad resistance.", 1},
		{"caps-50", "Bottle Caps x50", "Starting currency.", 2},
		{"smoke-pellet", "Smoke Pellet", "Auto-escape one combat encounter.", 3},
		{"pawned-ring", "Pawned Wedding Ring", "You meant to come back for it.", 0},
	},
}

var medic = Class{
	ID:            "medic",
	Name:          "Medic",
	Tagline:       "Chems, bandages, last-ditch miracles.",
	Flavor:        "You learned anatomy in a Vault and triage in a tent. Out here, the difference between sick and dead is whoever shows up with a stim first.",
	MentorNPCID:   "doc-vega",
	SignatureVerb: "stim",
	GatingSkill:   "survival",
	SkillBonuses:  []SkillBonus{{"survival", 1}},
	Kit: []KitItem{
		{"service-pistol", "Service Pistol", "1d6, sidearm.", 3},
		{"scalpel", "Scalpel", "1d4 melee, also crafts chems.", 1},
		{"lab-coat", "Lab Coat", "+1 defense, marks you as a medic.", 1},
		{"stimpak-3", "Stimpak x3", "Heal 25 HP each.", 2},
		{"bandages-5", "Bandages x5", "Heal 10 HP, stops bleeding.", 1},
		{"med-x", "Med-X", "+2 damage resist, 10 actions.", 2},
		{"buffout", "Buffout", "+20 max HP, 10 actions.", 2},
		{"empty-syringes-5", "Empty Syringes x5", "Crafting reagent.", 1},
		{"vault-tec-id", "Vault-Tec ID", "Bluffs Vault-related NPCs once.", 2},
		{"prewar-family-photo", "Pre-War Family Photo", "Two kids, a dog, sun.", 0},
	},
}

var ghoul = Class{
	ID:            "ghoul",
	Name:          "Ghoul",
	Tagline:       "Older than the bombs. Hungrier than them too.",
	Flavor:        "The radiation didn't kill you, it kept you. You remember the world before, mostly. The Settlers won't, and they'll spit when they see your face. The Collective will not.",
	MentorNPCID:   "mother-ash",
	SignatureVerb: "rad-feed",
	GatingSkill:   "scavenging",
	SkillBonuses:  []SkillBonus{{"scavenging", 1}},
	StartingRep: []RepDelta{
		{"ghoul-collective", 10},
		{"settlers", -10},
	},
	Kit: []KitItem{
		{"bone-club", "Bone Club", "1d6 melee.", 2},
		{"salvaged-pistol", "Salvaged Pistol", "1d6 sidearm.", 3},
		{"tattered-hood", "Tattered Hood", "+1 defense, hides ghoul features.", 1},
		{"glowing-trinket", "Glowing Trinket", "Lights dark rooms.", 2},
		{"rad-chunks-5", "Rad-Chunks x5", "rad-feed reagent: heal 15 HP each.", 1},
		{"junk-food-5", "Junk Food x5", "Heal 5 HP (irradiated).", 1},
		{"old-coin-pouch", "Old-World Coin Pouch", "30 caps.", 2},
		{"reinforced-wraps", "Reinforced Wrappings", "+1 defense, +1 fire resist.", 2},
		{"ancient-grudge", "Ancient Grudge", "+5 damage on first attack vs Ironclad.", 4},
		{"prewar-locket", "Pre-War Locket", "It still opens. The picture is gone.", 0},
	},
}
