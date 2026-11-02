package tts

import (
	"context"
	"time"
)

// Adapter exposes Edge as a simple Speak(url, duration) helper for the pipeline.
type Adapter struct {
	Edge *Edge
}

func (a Adapter) Speak(ctx context.Context, id, text string) (string, time.Duration, error) {
	if a.Edge == nil {
		return "", 0, context.Canceled
	}
	res, err := a.Edge.Speak(ctx, id, text)
	if err != nil {
		return "", 0, err
	}
	return res.URL, res.Duration, nil
}
