package clipboard

import "testing"

func TestRingAddDedup(t *testing.T) {
	r := New(3)
	a := r.Add("hello")
	if a.ID == 0 || a.Preview != "hello" {
		t.Fatalf("Add = %+v", a)
	}
	b := r.Add("hello")
	if b.ID != a.ID {
		t.Errorf("re-add should dedup, got id %d want %d", b.ID, a.ID)
	}
	if len(r.List()) != 1 {
		t.Errorf("List len = %d, want 1", len(r.List()))
	}
}

func TestRingPinAndClear(t *testing.T) {
	r := New(10)
	r.Add("a")
	p := r.Add("b")
	r.Add("c")
	if !r.SetPinned(p.ID, true) {
		t.Fatal("SetPinned failed")
	}
	if n := r.Clear(); n != 2 {
		t.Errorf("Clear removed %d, want 2", n)
	}
	items := r.List()
	if len(items) != 1 || items[0].Text != "b" {
		t.Errorf("after clear: %+v", items)
	}
}

func TestRingEvict(t *testing.T) {
	r := New(2)
	r.Add("one")
	r.Add("two")
	r.Add("three")
	items := r.List()
	if len(items) != 2 || items[0].Text != "three" || items[1].Text != "two" {
		t.Errorf("evict result: %+v", items)
	}
}

func TestRingDelete(t *testing.T) {
	r := New(5)
	it := r.Add("x")
	if !r.Delete(it.ID) {
		t.Error("Delete should report true")
	}
	if r.Delete(it.ID) {
		t.Error("second Delete should report false")
	}
}
