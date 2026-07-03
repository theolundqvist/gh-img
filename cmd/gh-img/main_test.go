package main

import (
	"reflect"
	"testing"
)

func TestFlagsFirst(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"flag before url", []string{"-o", "f", "URL"}, []string{"-o", "f", "URL"}},
		{"flag after url", []string{"URL", "-o", "f"}, []string{"-o", "f", "URL"}},
		{"equals form", []string{"URL", "-o=f"}, []string{"-o=f", "URL"}},
		{"o last with no value", []string{"URL", "-o"}, []string{"-o", "URL"}},
		{"token after url", []string{"URL", "--token", "t"}, []string{"--token", "t", "URL"}},
		{"url only", []string{"URL"}, []string{"URL"}},
	}
	for _, c := range cases {
		if got := flagsFirst(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: flagsFirst(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}
