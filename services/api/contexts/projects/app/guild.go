package app

import "context"

type guildKey struct{}

// InGuild scopes ctx to the Guild: through it, the Projects, Environments
// and Applications of every other Guild are not found. A request acts in its
// Current guild; background work (deploy workers, webhooks, probes) passes
// an unscoped context and sees every Guild.
func InGuild(ctx context.Context, guildID uint64) context.Context {
	return context.WithValue(ctx, guildKey{}, guildID)
}

// GuildOf is the Guild ctx is scoped to, if any.
func GuildOf(ctx context.Context) (uint64, bool) {
	id, ok := ctx.Value(guildKey{}).(uint64)
	return id, ok
}
