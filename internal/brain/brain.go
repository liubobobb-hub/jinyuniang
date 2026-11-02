package brain

import "context"

// Result is what the pet will say for one danmaku line.
type Result struct {
	Reply    string
	Mock     bool
	Degraded bool
	Err      error
}

// Brain turns a viewer line into a short spoken reply.
type Brain interface {
	Reply(ctx context.Context, user, text string) Result
	Mode() string
}
