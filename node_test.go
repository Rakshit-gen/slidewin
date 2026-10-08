package slidewin

import (
	"errors"
	"testing"
	"time"
)

func TestNodeIncrNeedsSet(t *testing.T) {
	n := NewNode(nil)
	if _, err := n.Incr("a", 1); !errors.Is(err, ErrMiss) {
		t.Fatalf("incr on missing key: %v", err)
	}
	n.Set("a", 5, 0)
	if v, err := n.Incr("a", 2); err != nil || v != 7 {
		t.Fatalf("got %d, %v", v, err)
	}
}

func TestNodeExpiry(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	n := NewNode(c.now)
	n.Set("a", 1, time.Second)
	n.Incr("a", 1)
	c.advance(999 * time.Millisecond)
	if v, _ := n.Get("a"); v != 2 {
		t.Fatalf("before expiry: got %d", v)
	}
	c.advance(time.Millisecond)
	if _, err := n.Get("a"); !errors.Is(err, ErrMiss) {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestNodeGetMulti(t *testing.T) {
	n := NewNode(nil)
	n.Set("a", 1, 0)
	n.Set("b", 2, 0)
	got := n.GetMulti("a", "b", "c")
	if len(got) != 2 || got["a"] != 1 || got["b"] != 2 {
		t.Fatalf("got %v", got)
	}
}
