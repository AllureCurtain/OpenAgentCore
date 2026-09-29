//go:build unix

package claudesdk

import (
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/internal/modeltransport"
)

func TestConvertedExecutorRejectsFutureImages(t *testing.T) {
	config, req := persistentConfig(t, "complete")
	req.AgentOptions["model_provider"] = map[string]any{"protocol": "chat_completions", "base_url": "https://model.invalid/v1", "api_key": "private-sentinel"}
	resource, err := NewExecutorFactory(config)(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resource.Close(t.Context())
	e := resource.(*executor)
	if !e.modelRoute.Converted() {
		t.Fatal("conversion route was not retained")
	}
	out := make(chan proto.Envelope, 20)
	turn, err := e.StartTurn(t.Context(), "text-first", proto.TextInput("hello"), out)
	if err != nil {
		t.Fatal(err)
	}
	settlement, err := turn.AwaitSettlement(t.Context())
	if err != nil || !settlement.Reusable {
		t.Fatalf("text did not settle: %v", err)
	}
	for range out {
	}
	ref := "data:image/png;base64,AA=="
	input := proto.MessageInput{{Content: []proto.InputContent{{Type: "input_image", ImageURL: &ref}}}}
	out = make(chan proto.Envelope, 20)
	if next, err := e.StartTurn(t.Context(), "image-next", input, out); next != nil || err != modeltransport.ErrUnsupported {
		t.Fatalf("image acquired native ownership: %v", err)
	}
	s := turn.(*session)
	if err := s.SteerWithReceipt(t.Context(), proto.PromptSteerPayload{InputID: "image", Input: input}, nil); err != modeltransport.ErrUnsupported {
		t.Fatalf("steer accepted image: %v", err)
	}
	if err := s.SubmitFunctionResult(t.Context(), proto.FunctionResultPayload{CallID: "call", DeliveryID: "delivery", Success: true, Content: input[0].Content}); err != modeltransport.ErrUnsupported {
		t.Fatalf("tool image accepted: %v", err)
	}
	close(out)
}
