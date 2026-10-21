package customerportal

// ApplyFulfillmentCapabilityDependencies derives customer-specific capabilities
// without changing the ordinary configuration or a shared live template.
func ApplyFulfillmentCapabilityDependencies(configured []Capability, modes []string) []Capability {
	required := false
	for _, mode := range modes {
		if mode == "stock" || mode == "on_demand" {
			required = true
		}
	}
	out := make([]Capability, len(configured))
	found := false
	for i, capability := range configured {
		out[i] = capability
		out[i].Config = make(map[string]any, len(capability.Config)+1)
		for key, value := range capability.Config {
			out[i].Config[key] = value
		}
		if capability.Code != CapabilityProductOrder {
			continue
		}
		found = true
		// Dependency metadata is server-owned, never a saved authorization flag.
		delete(out[i].Config, "required_by_fulfillment")
		if required {
			out[i].Enabled = true
			out[i].Config["required_by_fulfillment"] = true
		}
	}
	if required && !found {
		out = append(out, Capability{Code: CapabilityProductOrder, Enabled: true, Config: map[string]any{"required_by_fulfillment": true}})
	}
	return out
}
