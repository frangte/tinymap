package content

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Story flags (docs/design/15-flags-outcomes.md). A flag is a fact about a character that a quest sets when it is
// done (rewards.flags), or when it ends in an outcome (the outcome's flags, and the flag "<quest id>:<outcome id>"
// that every outcome sets by itself). Quests (requires_flags, unless_flags), dialogue answers (if_flags,
// unless_flags) and lines (say keys "flag:<id>") read them. Flags are declared nowhere: validation only needs
// every flag that is read to be set by some quest. In v1 a flag is never cleared.

const (
	// SayFlagPrefix starts a say key that is a line for characters holding a flag: "flag:<flag id>"
	SayFlagPrefix = "flag:"
	// flagsMax bounds the flags one quest or outcome sets, or one list of flags a quest or answer reads
	flagsMax = 6
)

var (
	// flagPattern is a flag id that can be set: lower_snake_case words joined by dots
	flagPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*(\.[a-z0-9]+(_[a-z0-9]+)*)*$`)
	// readFlagPattern is a flag id that can be read: the same, or the outcome form "<quest id>:<outcome id>"
	readFlagPattern = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*(\.[a-z0-9]+(_[a-z0-9]+)*)*(:[a-z0-9]+(_[a-z0-9]+)*)?$`)
)

// OutcomeFlag is the flag a quest sets when it ends with that outcome.
func OutcomeFlag(quest, outcome string) string { return quest + ":" + outcome }

// FlagSetters is, for every flag some quest sets, the ids of the quests that set it.
func (c *Catalog) FlagSetters() map[string][]string {
	out := map[string][]string{}
	add := func(flag, quest string) {
		if !slices.Contains(out[flag], quest) {
			out[flag] = append(out[flag], quest)
		}
	}
	for _, q := range c.Quests {
		for _, f := range q.Rewards.Flags {
			add(f, q.ID)
		}
		for _, o := range q.Outcomes {
			add(OutcomeFlag(q.ID, o.ID), q.ID)
			for _, f := range o.Flags {
				add(f, q.ID)
			}
		}
	}
	return out
}

// checkFlagsWritten checks the flags a quest or outcome sets: plain ids (the outcome form is set by the engine).
func checkFlagsWritten(what string, flags []string) error {
	if len(flags) > flagsMax {
		return fmt.Errorf("%s: at most %d flags", what, flagsMax)
	}
	for i, f := range flags {
		if !flagPattern.MatchString(f) {
			return fmt.Errorf("%s: flag %q must be lower_snake_case words joined by dots", what, f)
		}
		if slices.Contains(flags[:i], f) {
			return fmt.Errorf("%s: flag %q is listed twice", what, f)
		}
	}
	return nil
}

// checkFlagsRead checks the flags a quest, answer or line reads: well formed, and set by some quest (other than
// `not`, when given: a quest that waits for a flag only it sets would never be offered).
func checkFlagsRead(what string, flags []string, setters map[string][]string, not string) error {
	if len(flags) > flagsMax {
		return fmt.Errorf("%s: at most %d flags", what, flagsMax)
	}
	for _, f := range flags {
		if !readFlagPattern.MatchString(f) {
			return fmt.Errorf("%s: flag %q is not a flag id (words_like_this, or quest.id:outcome_id)", what, f)
		}
		by := setters[f]
		if len(by) == 0 {
			return fmt.Errorf("%s: flag %q is read but no quest sets it (rewards.flags, or an outcome's flags)", what, f)
		}
		if not != "" && len(by) == 1 && by[0] == not {
			return fmt.Errorf("%s: flag %q is only set by this quest", what, f)
		}
	}
	return nil
}

// sayFlag is the flag id of a say key "flag:<id>", and whether the key is one.
func sayFlag(key string) (string, bool) { return strings.CutPrefix(key, SayFlagPrefix) }
