package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUTF8Reader_valid(t *testing.T) {
	content := "バラク・フセイン・オバマ2世（Barack Hussein Obama II[2] [bəˈrɑːk huːˈseɪn oʊˈbɑːmə] (聞く)、1961年8月4日 - ）は、アメリカ合衆国の政治家。民主党所属。上院議員（1期）、イリノイ州上院議員（3期）、第44代アメリカ合衆国大統領を歴任した。"
	for offset, r := range content {
		if r == utf8.RuneError {
			t.Errorf("Invalid UTF8 string right after: %q", content[:offset])
			return
		}
	}
outer:
	for size := 8; size < 100; size++ {
		r := newUTF8AwareReader(bytes.NewBufferString(content))
		buf := make([]byte, size)
		var segments []string
		for {
			n, err := r.Read(buf)
			if err == io.EOF {
				break
			}
			for _, c := range string(buf[:n]) {
				if c == utf8.RuneError {
					t.Errorf("Invalid segment: %q", buf[:n])
					continue outer
				}
			}
			segments = append(segments, string(buf[:n]))
		}
		data := strings.Join(segments, "")
		if data != content {
			t.Errorf("Unexpected data: %s", data)
		}
	}
}

func TestUTF8Reader_invalid(t *testing.T) {
	// "日本語" in SJIS encoding.
	r := newUTF8AwareReader(bytes.NewBufferString("\x93\xfa\x96{\x8c\xea"))
	var buf [1024]byte
	n, err := r.Read(buf[:])
	fail := false
	if n != 4 {
		t.Errorf("Unexpected n: %d", n)
		fail = true
	}
	if buf[n-1] != '{' {
		t.Errorf("Unexpected last char: '%c' (0x%x)", buf[n-1], buf[n-1])
		fail = true
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		fail = true
	}
	if fail {
		return
	}
	n, err = r.Read(buf[4:])
	if n != 2 {
		t.Errorf("Unexpected n: %d", n)
		fail = true
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		fail = true
	}
	if fail {
		return
	}
	n, err = r.Read(buf[:])
	if n != 0 || err != io.EOF {
		t.Errorf("Unexpected (n, err): (%v, %v)", n, err)
	}
}
