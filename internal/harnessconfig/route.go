package harnessconfig

import "github.com/MiniMax-AI-Dev/parsar/internal/modeltransport"

func (c Configuration) AcceptsHarnessConfig() bool { return c.ValidateNativeConfig != nil }
func (c Configuration) NativeProtocols() []modeltransport.Protocol {
	var protocols []modeltransport.Protocol
	for _, provider := range c.Providers {
		if provider.Native {
			protocols = append(protocols, modeltransport.Protocol(provider.Protocol))
		}
	}
	return protocols
}
func (c Configuration) ValidateRoute(protocol string, required modeltransport.Requirements) error {
	if err := c.ValidateProtocol(protocol); err != nil {
		return err
	}
	route, err := modeltransport.ResolveRoute(modeltransport.Protocol(protocol), c.NativeProtocols())
	if err != nil {
		return err
	}
	return route.Validate(required)
}
