// Package claudesdk owns the qualified Claude SDK provider declaration.
package claudesdk

import "github.com/MiniMax-AI-Dev/parsar/internal/harnessconfig"

func Configuration() harnessconfig.Configuration {
	return harnessconfig.Configuration{ValidateNativeConfig: validateNativeConfig, Providers: []harnessconfig.Provider{
		{Native: true, Protocol: "anthropic"},
		{Protocol: "responses"},
		{Protocol: "chat_completions"},
	}}
}
