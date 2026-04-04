package busd

// MapMudEvent maps a mud BUSD topic and its payload to a game.action name.
// Returns the action name and true if the event should be forwarded to gamification.
// Returns "", false if the event should be ignored.
func MapMudEvent(topic string, payload map[string]any) (string, bool) {
	switch topic {
	case "mud.combat.ended":
		outcome, _ := payload["outcome"].(string)
		switch outcome {
		case "won":
			return "combat.won", true
		case "lost":
			return "combat.lost", true
		}
		return "", false
	case "mud.room.entered":
		first, _ := payload["first"].(bool)
		if !first {
			return "", false
		}
		return "room.explored", true
	case "mud.hack.success":
		return "hack.success", true
	case "mud.trade.completed":
		return "trade.completed", true
	case "mud.craft.completed":
		return "craft.completed", true
	case "mud.lock.picked":
		return "lock.picked", true
	case "mud.player.died":
		return "player.died", true
	default:
		return "", false
	}
}
