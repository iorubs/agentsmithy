// Package models is the config-time catalog of model-provider kinds.
//
// Each provider kind (`openai`, `anthropic`, `google`, `bedrock`,
// `vertex`, `borrowed`) lives in its own file. New picks the right
// constructor by config.Provider; in-tree providers return an
// adk model.LLM the runtime hands to llmagent.New.
//
// This package owns the full client/wire: providers construct their
// own SDK clients and implement model.LLM directly. The runtime
// stays out of provider details; it just calls New and wires the
// returned LLM into the agent.
package models

import (
	"context"
	"fmt"

	"github.com/iorubs/agentsmithy/internal/config"
	adkmodel "google.golang.org/adk/model"
)

// LLM is the agent-side interface providers must satisfy. It is
// ADK's model.LLM verbatim; aliasing keeps callers off the ADK
// import while preserving the contract llmagent.New consumes.
type LLM = adkmodel.LLM

// New builds an LLM for the model entry referenced by ref. It is
// the single entry point the runtime uses; the provider files in
// this package supply the per-kind constructors.
func New(ctx context.Context, ref config.ModelRef, entry config.ModelEntry) (LLM, error) {
	switch ref.Provider {
	case config.ProviderOpenAI:
		return newOpenAI(entry)
	case config.ProviderAnthropic:
		return newAnthropic(entry)
	case config.ProviderGoogle:
		return newGoogle(ctx, entry)
	case config.ProviderBedrock:
		return newBedrock(entry)
	case config.ProviderVertex:
		return newVertex(entry)
	case config.ProviderBorrowed:
		return newBorrowed(entry)
	default:
		return nil, fmt.Errorf("unknown provider %q", ref.Provider)
	}
}
