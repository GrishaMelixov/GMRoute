package trie

import "testing"

func TestLookup_ExactMatch(t *testing.T) {
	tr := New[string]()
	tr.Add("google.com", "upstream")

	val, ok := tr.Lookup("google.com")
	if !ok || val != "upstream" {
		t.Errorf("expected upstream, got %q ok=%v", val, ok)
	}
}

func TestLookup_SubdomainInheritsParentRule(t *testing.T) {
	tr := New[string]()
	tr.Add("google.com", "upstream")

	tests := []string{
		"mail.google.com",
		"www.google.com",
		"s3.cdn.google.com",
	}
	for _, domain := range tests {
		val, ok := tr.Lookup(domain)
		if !ok || val != "upstream" {
			t.Errorf("domain %q: expected upstream, got %q ok=%v", domain, val, ok)
		}
	}
}

func TestLookup_MoreSpecificRuleWins(t *testing.T) {
	tr := New[string]()
	tr.Add("google.com", "upstream")
	tr.Add("mail.google.com", "direct")

	val, ok := tr.Lookup("mail.google.com")
	if !ok || val != "direct" {
		t.Errorf("expected direct (more specific rule), got %q ok=%v", val, ok)
	}

	val, ok = tr.Lookup("www.google.com")
	if !ok || val != "upstream" {
		t.Errorf("expected upstream (parent rule), got %q ok=%v", val, ok)
	}
}

func TestLookup_NotFound(t *testing.T) {
	tr := New[string]()
	tr.Add("google.com", "upstream")

	_, ok := tr.Lookup("example.com")
	if ok {
		t.Error("expected not found for unknown domain")
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	tr := New[string]()
	tr.Add("Google.COM", "upstream")

	val, ok := tr.Lookup("google.com")
	if !ok || val != "upstream" {
		t.Errorf("expected case-insensitive match, got %q ok=%v", val, ok)
	}
}

func TestLookup_EmptyTrie(t *testing.T) {
	tr := New[int]()

	_, ok := tr.Lookup("anything.com")
	if ok {
		t.Error("expected not found in empty trie")
	}
}
