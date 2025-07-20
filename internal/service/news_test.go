package service

import "testing"

func TestNewsService_Latest(t *testing.T) {
	s := NewNewsService()
	got := s.Latest()
	if len(got) == 0 {
		t.Fatalf("expected some news")
	}
}
