package domain

// Actor is who did something in work: a Member or an Agent, never both.
// The zero Actor is nobody: an account since deleted, or the system.
// RunID is the Run an Agent acts in, for a Run key's request: it is not
// part of who the Actor is and is never stored.
type Actor struct {
	MemberID uint64
	AgentID  uint64
	RunID    uint64
}

// ByMember is the Member as Actor; 0 is nobody.
func ByMember(id uint64) Actor { return Actor{MemberID: id} }

// ByAgent is the Agent as Actor; 0 is nobody.
func ByAgent(id uint64) Actor { return Actor{AgentID: id} }

// None is true for nobody.
func (a Actor) None() bool { return a.MemberID == 0 && a.AgentID == 0 }

// Is is true when both are the same Member or the same Agent; nobody is
// never anybody.
func (a Actor) Is(b Actor) bool {
	return !a.None() && a.MemberID == b.MemberID && a.AgentID == b.AgentID
}
