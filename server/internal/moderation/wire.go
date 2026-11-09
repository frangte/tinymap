package moderation

import "saigon-noir/server/internal/protocol"

// Server-to-client messages of moderation (server/README.md, Moderation).

const (
	typeNotice   = "mod_notice"
	typeReported = "reported"
)

// noticeMsg is a line for one player's chat log: a command's answer, a mute's end, a report received. Text is
// Vietnamese; EN is the English of what players are told (replies to moderators' commands have none).
type noticeMsg struct {
	T    string `json:"t"`
	Text string `json:"text"`
	EN   string `json:"en,omitempty"`
}

// reportedMsg confirms a report was stored.
type reportedMsg struct {
	T string `json:"t"`
	N uint32 `json:"n"`
}

func encode(v any) []byte { return protocol.Encode(v) }
