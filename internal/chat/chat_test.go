package chat

import (
	"testing"
)

func TestSay(t *testing.T) {
	t.Run("empty args returns usage", func(t *testing.T) {
		res := Say("alice", nil)
		if res.Output != "say <message>" {
			t.Fatalf("expected usage, got %q", res.Output)
		}
		if len(res.ChatMessages) != 0 {
			t.Fatal("expected no chat messages for empty args")
		}
	})

	t.Run("single word", func(t *testing.T) {
		res := Say("alice", []string{"hello"})
		if len(res.ChatMessages) != 1 {
			t.Fatalf("expected 1 chat message, got %d", len(res.ChatMessages))
		}
		cm := res.ChatMessages[0]
		if cm.Type != "say" {
			t.Fatalf("expected type say, got %q", cm.Type)
		}
		if cm.Sender != "alice" {
			t.Fatalf("expected sender alice, got %q", cm.Sender)
		}
		if cm.Body != "hello" {
			t.Fatalf("expected body hello, got %q", cm.Body)
		}
		if cm.Target != "" {
			t.Fatalf("expected empty target, got %q", cm.Target)
		}
	})

	t.Run("multiple words joined", func(t *testing.T) {
		res := Say("bob", []string{"hello", "world"})
		if res.ChatMessages[0].Body != "hello world" {
			t.Fatalf("expected joined body, got %q", res.ChatMessages[0].Body)
		}
	})
}

func TestShout(t *testing.T) {
	t.Run("empty args returns usage", func(t *testing.T) {
		res := Shout("alice", nil)
		if res.Output != "shout <message>" {
			t.Fatalf("expected usage, got %q", res.Output)
		}
		if len(res.ChatMessages) != 0 {
			t.Fatal("expected no chat messages for empty args")
		}
	})

	t.Run("broadcasts shout", func(t *testing.T) {
		res := Shout("alice", []string{"hey", "everyone"})
		if len(res.ChatMessages) != 1 {
			t.Fatalf("expected 1 chat message, got %d", len(res.ChatMessages))
		}
		cm := res.ChatMessages[0]
		if cm.Type != "shout" {
			t.Fatalf("expected type shout, got %q", cm.Type)
		}
		if cm.Body != "hey everyone" {
			t.Fatalf("expected body 'hey everyone', got %q", cm.Body)
		}
	})
}

func TestWhisper(t *testing.T) {
	t.Run("no args returns usage", func(t *testing.T) {
		res := Whisper("alice", nil)
		if res.Output != "whisper <player> <message>" {
			t.Fatalf("expected usage, got %q", res.Output)
		}
	})

	t.Run("only target no message returns usage", func(t *testing.T) {
		res := Whisper("alice", []string{"bob"})
		if res.Output != "whisper <player> <message>" {
			t.Fatalf("expected usage, got %q", res.Output)
		}
	})

	t.Run("sends whisper", func(t *testing.T) {
		res := Whisper("alice", []string{"bob", "hey", "there"})
		if len(res.ChatMessages) != 1 {
			t.Fatalf("expected 1 chat message, got %d", len(res.ChatMessages))
		}
		cm := res.ChatMessages[0]
		if cm.Type != "whisper" {
			t.Fatalf("expected type whisper, got %q", cm.Type)
		}
		if cm.Sender != "alice" {
			t.Fatalf("expected sender alice, got %q", cm.Sender)
		}
		if cm.Target != "bob" {
			t.Fatalf("expected target bob, got %q", cm.Target)
		}
		if cm.Body != "hey there" {
			t.Fatalf("expected body 'hey there', got %q", cm.Body)
		}
		if res.Output != "[to bob] hey there" {
			t.Fatalf("expected output '[to bob] hey there', got %q", res.Output)
		}
	})
}
