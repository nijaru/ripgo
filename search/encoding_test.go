package search_test

import (
	"bytes"
	"testing"

	"github.com/nijaru/ripgo/pattern"
	"github.com/nijaru/ripgo/search"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

func TestLookupEncoding(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"", false},
		{"auto", false},
		{"none", false},
		{"binary", false},
		{"raw", false},
		{"utf-8", false},
		{"utf8", false},
		{"utf-16", false},
		{"utf-16le", false},
		{"utf-16be", false},
		{"latin1", false},
		{"iso-8859-1", false},
		{"windows-1252", false},
		{"cp1252", false},
		{"shift_jis", false},
		{"gbk", false},
		{"euc-jp", false},
		{"nonexistent-encoding-xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := search.LookupEncoding(tt.name)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LookupEncoding(%q) err = %v, wantErr = %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestDecodeDataAutoBOM(t *testing.T) {
	// UTF-16LE with BOM
	rawLE, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte("hello world\nline 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	outLE, modified, err := search.DecodeData(rawLE, "auto")
	if err != nil {
		t.Fatalf("DecodeData LE auto failed: %v", err)
	}
	if !modified {
		t.Errorf("expected modified=true for UTF-16LE")
	}
	if string(outLE) != "hello world\nline 2\n" {
		t.Errorf("got %q, want 'hello world\\nline 2\\n'", string(outLE))
	}

	// UTF-16BE with BOM
	rawBE, err := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder().Bytes([]byte("hello world\nline 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	outBE, modified, err := search.DecodeData(rawBE, "auto")
	if err != nil {
		t.Fatalf("DecodeData BE auto failed: %v", err)
	}
	if !modified {
		t.Errorf("expected modified=true for UTF-16BE")
	}
	if string(outBE) != "hello world\nline 2\n" {
		t.Errorf("got %q, want 'hello world\\nline 2\\n'", string(outBE))
	}

	// UTF-8 with BOM
	rawUTF8BOM := append([]byte{0xef, 0xbb, 0xbf}, []byte("hello world\n")...)
	outUTF8BOM, _, err := search.DecodeData(rawUTF8BOM, "auto")
	if err != nil {
		t.Fatalf("DecodeData UTF-8 BOM auto failed: %v", err)
	}
	if string(outUTF8BOM) != "hello world\n" {
		t.Errorf("got %q, want 'hello world\\n'", string(outUTF8BOM))
	}

	// Regular UTF-8 without BOM
	plain := []byte("hello world\n")
	outPlain, modified, err := search.DecodeData(plain, "auto")
	if err != nil {
		t.Fatalf("DecodeData plain failed: %v", err)
	}
	if modified {
		t.Errorf("expected modified=false for plain UTF-8")
	}
	if !bytes.Equal(outPlain, plain) {
		t.Errorf("got %q, want %q", outPlain, plain)
	}
}

func TestDecodeDataExplicitEncoding(t *testing.T) {
	// Latin-1 (ISO-8859-1) with characters like 'é' (0xE9 in Latin1, 0xC3 0xA9 in UTF-8)
	rawLatin1, err := charmap.ISO8859_1.NewEncoder().Bytes([]byte("café résumé\n"))
	if err != nil {
		t.Fatal(err)
	}
	outLatin1, _, err := search.DecodeData(rawLatin1, "latin1")
	if err != nil {
		t.Fatalf("DecodeData latin1 failed: %v", err)
	}
	if string(outLatin1) != "café résumé\n" {
		t.Errorf("got %q, want 'café résumé\\n'", string(outLatin1))
	}

	// UTF-16LE without BOM
	rawLEnoBOM, err := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder().Bytes([]byte("unicode test"))
	if err != nil {
		t.Fatal(err)
	}
	outLE, _, err := search.DecodeData(rawLEnoBOM, "utf-16le")
	if err != nil {
		t.Fatalf("DecodeData utf-16le failed: %v", err)
	}
	if string(outLE) != "unicode test" {
		t.Errorf("got %q, want 'unicode test'", string(outLE))
	}

	// Raw / none mode
	outNone, modified, err := search.DecodeData(rawLEnoBOM, "none")
	if err != nil {
		t.Fatalf("DecodeData none failed: %v", err)
	}
	if modified {
		t.Errorf("expected modified=false for none mode")
	}
	if !bytes.Equal(outNone, rawLEnoBOM) {
		t.Errorf("none mode modified raw bytes")
	}
}

func TestSearchWithEncoding(t *testing.T) {
	m, err := pattern.New(pattern.Config{Pattern: "match_me"})
	if err != nil {
		t.Fatal(err)
	}

	// Create UTF-16LE file content with BOM
	utf16Data, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte("line 1\nmatch_me here\nline 3\n"))
	if err != nil {
		t.Fatal(err)
	}

	// Search with default auto encoding
	sAuto := search.NewSearcher(nil, search.Config{}, m)
	resAuto, err := sAuto.SearchBytes(utf16Data, "test.txt")
	if err != nil {
		t.Fatalf("SearchBytes auto failed: %v", err)
	}
	if resAuto.Binary {
		t.Fatalf("expected resAuto not binary, got binary")
	}
	if len(resAuto.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(resAuto.Matches))
	}
	if resAuto.Matches[0].Line != 2 {
		t.Errorf("expected match on line 2, got line %d", resAuto.Matches[0].Line)
	}

	// Search with explicit latin1 on ISO-8859-1 encoded text
	latin1Data, err := charmap.ISO8859_1.NewEncoder().Bytes([]byte("prefix match_me suffix\n"))
	if err != nil {
		t.Fatal(err)
	}
	sLatin1 := search.NewSearcher(nil, search.Config{Encoding: "latin1"}, m)
	resLatin1, err := sLatin1.SearchBytes(latin1Data, "latin1.txt")
	if err != nil {
		t.Fatalf("SearchBytes latin1 failed: %v", err)
	}
	if len(resLatin1.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(resLatin1.Matches))
	}
}
