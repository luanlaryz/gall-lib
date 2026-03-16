package runtime

import "fmt"

func ExampleNewEngine_reasoningDisabled() {
	eng := NewEngine().(*engine)
	fmt.Println(eng.reasoning == nil)
	// Output: true
}

func ExampleNewEngine_reasoningThinkOnly() {
	cfg := DefaultReasoningConfig()
	cfg.Enabled = true
	cfg.Mode = ReasoningModeThinkOnly

	eng := NewEngine(WithReasoningConfig(cfg)).(*engine)
	descriptors := eng.reasoning.registry.List()
	fmt.Println(len(descriptors), descriptors[0].Name)
	// Output: 1 reasoning.think
}
