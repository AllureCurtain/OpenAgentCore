package codex

import (
	"testing"

	"github.com/MiniMax-AI-Dev/parsar/internal/agentdaemon/proto"
	"github.com/MiniMax-AI-Dev/parsar/internal/modeltransport"
)

func TestConvertedExecutorRejectsFutureImageBeforeOwnership(t *testing.T) {
	e, root := executorFixture(t, "complete")
	route := modeltransport.Route{Native: modeltransport.Responses, Upstream: modeltransport.Anthropic}
	e.prepared.plan.ModelRoute = route
	e.prepared.session.modelRoute = route
	out := make(chan proto.Envelope, 20)
	turn, err := e.StartTurn(t.Context(), "text-first", proto.TextInput("hello"), out)
	if err != nil {
		t.Fatal(err)
	}
	if !awaitExecutorTurn(t, turn, out).Reusable {
		t.Fatal("text did not settle")
	}
	before := len(preparationFrames(t, root))
	ref := "data:image/png;base64,AA=="
	input := proto.MessageInput{{Content: []proto.InputContent{{Type: "input_image", ImageURL: &ref}}}}
	out = make(chan proto.Envelope, 20)
	if next, err := e.StartTurn(t.Context(), "image-next", input, out); next != nil || err != modeltransport.ErrUnsupported {
		t.Fatalf("image acquired output/native ownership: %v", err)
	}
	session := turn.(*Session)
	if err := session.SteerWithReceipt(t.Context(), proto.PromptSteerPayload{InputID: "image", Input: input}, nil); err != modeltransport.ErrUnsupported {
		t.Fatalf("steer accepted image: %v", err)
	}
	if err := session.SubmitFunctionResult(t.Context(), proto.FunctionResultPayload{CallID: "call", DeliveryID: "delivery", Success: true, Content: input[0].Content}); err != modeltransport.ErrUnsupported {
		t.Fatalf("tool image accepted: %v", err)
	}
	if len(preparationFrames(t, root)) != before {
		t.Fatal("rejected content reached native process")
	}
	close(out)
}
