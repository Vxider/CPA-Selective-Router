package plugin

import (
	"context"

	"selective-model-router/core"
)

type Config struct {
	Visual    VisualConfig
	WebSearch WebSearchConfig
}

type VisualConfig struct {
	Enabled   bool
	Provider  core.Provider
	Model     string
	MaxRounds int
	MaxTokens int
}

type WebSearchConfig struct {
	Enabled   bool
	Provider  core.Provider
	Model     string
	MaxRounds int
	MaxTokens int
}

type Wrapper interface {
	Wrap(ctx context.Context, upstream core.Provider) (core.Provider, error)
}

type Composite struct {
	wrappers []Wrapper
}

func NewComposite(wrappers ...Wrapper) *Composite {
	return &Composite{wrappers: wrappers}
}

func (c *Composite) Wrap(ctx context.Context, upstream core.Provider) (core.Provider, error) {
	var err error
	current := upstream
	for _, wrapper := range c.wrappers {
		if wrapper == nil {
			continue
		}
		current, err = wrapper.Wrap(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}
