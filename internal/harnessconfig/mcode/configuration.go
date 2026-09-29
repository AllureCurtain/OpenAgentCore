// Package mcode owns the qualified MiniMax Code provider declaration.
package mcode

import "github.com/MiniMax-AI-Dev/parsar/internal/harnessconfig"

func Configuration() harnessconfig.Configuration {
	return harnessconfig.Configuration{Providers: []harnessconfig.Provider{
		{Native: true, Protocol: "anthropic", RequiresTokenLimits: true},
		{Native: true, Protocol: "responses", RequiresTokenLimits: true},
		{Native: true, Protocol: "chat_completions", RequiresTokenLimits: true},
	}}
}
