package trie

import "strings"

type node[T any] struct {
	children map[string]*node[T]
	value    *T
}

type Trie[T any] struct {
	root *node[T]
}

func New[T any]() *Trie[T] {
	return &Trie[T]{root: &node[T]{children: make(map[string]*node[T])}}
}

func (t *Trie[T]) Add(domain string, value T) {
	parts := reverseParts(domain)
	cur := t.root

	for _, part := range parts {
		if cur.children == nil {
			cur.children = make(map[string]*node[T])
		}
		if _, ok := cur.children[part]; !ok {
			cur.children[part] = &node[T]{children: make(map[string]*node[T])}
		}
		cur = cur.children[part]
	}

	cur.value = &value
}

func (t *Trie[T]) Lookup(domain string) (T, bool) {
	parts := reverseParts(domain)
	cur := t.root

	var last *T

	for _, part := range parts {
		next, ok := cur.children[part]
		if !ok {
			break
		}
		cur = next
		if cur.value != nil {
			last = cur.value
		}
	}

	if last == nil {
		var zero T
		return zero, false
	}
	return *last, true
}

func reverseParts(domain string) []string {
	parts := strings.Split(strings.ToLower(domain), ".")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return parts
}
