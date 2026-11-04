package main

import (
	"context"
	"testing"

	pb "termium/client/pb"
)

func TestQueuedWheelTicksMergeWithoutReorderingOtherInput(t *testing.T) {
	d := &inputDispatcher{ctx: context.Background(), wake: make(chan struct{}, 1)}
	wheel := func(dy int32, x int32) browserOperation {
		return browserOperation{input: &pb.InputEvent{Kind: pb.InputKind_WHEEL_INPUT, DeltaY: dy, X: x, Y: 5, Generation: 3, TabId: "t"}}
	}
	for i := 0; i < 30; i++ {
		d.enqueue(wheel(60, int32(i)))
	}
	if len(d.pending) != 1 || d.pending[0].input.DeltaY != 30*60 || d.pending[0].input.X != 29 {
		t.Fatalf("30 ticks should merge into one wheel of 1800px at the latest position, got %d ops %+v", len(d.pending), d.pending[0].input)
	}
	d.enqueue(browserOperation{input: &pb.InputEvent{Kind: pb.InputKind_KEY_INPUT, Key: "j", Generation: 3, TabId: "t"}})
	d.enqueue(wheel(-60, 1))
	d.enqueue(wheel(-60, 1))
	if len(d.pending) != 3 || d.pending[1].input.Kind != pb.InputKind_KEY_INPUT || d.pending[2].input.DeltaY != -120 {
		t.Fatalf("a key between wheels must keep its place; got %d ops", len(d.pending))
	}
	other := wheel(60, 1)
	other.input.TabId = "other"
	d.enqueue(other)
	if len(d.pending) != 4 {
		t.Fatal("wheels for a different tab must not merge")
	}
}
