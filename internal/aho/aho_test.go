package aho

import "testing"

func TestNew_NilOnEmpty(t *testing.T) {
	m := New(nil)
	if m != nil {
		t.Fatal("New(nil) should return nil")
	}
	m = New([][]byte{})
	if m != nil {
		t.Fatal("New(empty) should return nil")
	}
	m = New([][]byte{{}, {}})
	if m != nil {
		t.Fatal("New(emptypatterns) should return nil")
	}
}

func TestMatchesAny_Basic(t *testing.T) {
	m := New([][]byte{[]byte("foo"), []byte("bar"), []byte("baz")})

	tests := []struct {
		data string
		want bool
	}{
		{"foo", true},
		{"bar", true},
		{"baz", true},
		{"hello world", false},
		{"foobar", true},
		{"a foo here", true},
		{"xbarx", true},
		{"no match", false},
	}
	for _, tt := range tests {
		got := m.MatchesAny([]byte(tt.data))
		if got != tt.want {
			t.Errorf("MatchesAny(%q) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

func TestMatchesAny_SinglePattern(t *testing.T) {
	m := New([][]byte{[]byte("hello")})
	if !m.MatchesAny([]byte("say hello world")) {
		t.Error("should match")
	}
	if m.MatchesAny([]byte("goodbye")) {
		t.Error("should not match")
	}
}

func TestMatchesAny_PrefixOverlap(t *testing.T) {
	m := New([][]byte{[]byte("abc"), []byte("abcd")})
	if !m.MatchesAny([]byte("xabcde")) {
		t.Error("should match 'abc' in 'xabcde'")
	}
	if !m.MatchesAny([]byte("xyzabcd")) {
		t.Error("should match 'abcd' in 'xyzabcd'")
	}
	if m.MatchesAny([]byte("ab")) {
		t.Error("should not match partial")
	}
}

func TestMatchesAny_SuffixOverlap(t *testing.T) {
	m := New([][]byte{[]byte("abc"), []byte("xabc")})
	if !m.MatchesAny([]byte("xabc")) {
		t.Error("should match")
	}
	if !m.MatchesAny([]byte("abc")) {
		t.Error("should match")
	}
}

func TestMatchesAny_EmptyData(t *testing.T) {
	m := New([][]byte{[]byte("a")})
	if m.MatchesAny(nil) {
		t.Error("nil data should not match")
	}
	if m.MatchesAny([]byte{}) {
		t.Error("empty data should not match")
	}
}

func TestMatchesAny_NilMachine(t *testing.T) {
	var m *Machine
	if m.MatchesAny([]byte("anything")) {
		t.Error("nil machine should never match")
	}
}

func TestMatchesAny_ConsecutivePatterns(t *testing.T) {
	m := New([][]byte{[]byte("foo"), []byte("bar")})
	if !m.MatchesAny([]byte("foobar")) {
		t.Error("should match both patterns in sequence")
	}
}

func BenchmarkMatchesAny_3Patterns(b *testing.B) {
	m := New([][]byte{[]byte("foo"), []byte("bar"), []byte("baz")})
	data := []byte("the quick brown fox jumps over the lazy dog")
	for b.Loop() {
		m.MatchesAny(data)
	}
}

func BenchmarkMatchesAny_3Patterns_NoMatch(b *testing.B) {
	m := New([][]byte{[]byte("xyz"), []byte("pqr"), []byte("mno")})
	data := []byte("the quick brown fox jumps over the lazy dog")
	for b.Loop() {
		m.MatchesAny(data)
	}
}
