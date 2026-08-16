package search

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// LookupEncoding resolves an encoding name to a supported encoding.Encoding.
// Case-insensitive, supporting common aliases (e.g. "utf-16le", "latin1", "windows-1252").
// Special names:
// - "" or "auto": returns (nil, nil) indicating automatic BOM sniffing.
// - "none", "binary", "raw": returns (encoding.Nop, nil) indicating raw byte scanning without BOM sniffing.
func LookupEncoding(name string) (encoding.Encoding, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" || name == "auto" {
		return nil, nil
	}
	if name == "none" || name == "binary" || name == "raw" {
		return encoding.Nop, nil
	}

	switch name {
	case "utf-8", "utf8":
		return unicode.UTF8, nil
	case "utf-16", "utf16":
		return unicode.UTF16(unicode.LittleEndian, unicode.UseBOM), nil
	case "utf-16le", "utf16le":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), nil
	case "utf-16be", "utf16be":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), nil
	case "latin1", "latin-1", "iso-8859-1", "iso8859-1", "iso88591":
		return charmap.ISO8859_1, nil
	case "latin2", "latin-2", "iso-8859-2", "iso8859-2":
		return charmap.ISO8859_2, nil
	case "windows-1252", "cp1252", "win1252", "win-1252":
		return charmap.Windows1252, nil
	case "windows-1251", "cp1251", "win1251", "win-1251":
		return charmap.Windows1251, nil
	case "windows-1250", "cp1250", "win1250", "win-1250":
		return charmap.Windows1250, nil
	case "ascii", "us-ascii":
		return charmap.ISO8859_1, nil
	}

	// Try HTML5 encoding index (common web names)
	if enc, err := htmlindex.Get(name); err == nil && enc != nil {
		return enc, nil
	}

	// Try IANA / MIME registries
	if enc, err := ianaindex.IANA.Encoding(name); err == nil && enc != nil {
		return enc, nil
	}
	if enc, err := ianaindex.MIME.Encoding(name); err == nil && enc != nil {
		return enc, nil
	}

	return nil, fmt.Errorf("unknown or unsupported encoding %q", name)
}

// DecodeData transcode raw file data to UTF-8 based on the configured encoding name.
// If encoding is empty or "auto", it sniffs for UTF-16LE, UTF-16BE, or UTF-8 BOMs.
// Returns the decoded data, a boolean indicating whether data was modified/copied, and any error.
func DecodeData(data []byte, encName string) ([]byte, bool, error) {
	encName = strings.TrimSpace(strings.ToLower(encName))

	// Raw mode: no BOM sniffing, no transcoding.
	if encName == "none" || encName == "binary" || encName == "raw" {
		return data, false, nil
	}

	// Auto mode: sniff for BOM.
	if encName == "" || encName == "auto" {
		if len(data) >= 2 {
			if data[0] == 0xff && data[1] == 0xfe {
				// UTF-16LE with BOM
				dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
				out, err := dec.Bytes(data)
				if err != nil {
					return nil, false, fmt.Errorf("decode utf-16le: %w", err)
				}
				return out, true, nil
			}
			if data[0] == 0xfe && data[1] == 0xff {
				// UTF-16BE with BOM
				dec := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
				out, err := dec.Bytes(data)
				if err != nil {
					return nil, false, fmt.Errorf("decode utf-16be: %w", err)
				}
				return out, true, nil
			}
		}
		if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
			// UTF-8 BOM: strip it
			return data[3:], false, nil
		}
		return data, false, nil
	}

	// Explicit encoding specified.
	enc, err := LookupEncoding(encName)
	if err != nil {
		return nil, false, err
	}
	if enc == nil || enc == encoding.Nop {
		if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
			return data[3:], false, nil
		}
		return data, false, nil
	}

	// For explicit encoding, apply BOMOverride so a BOM in the input takes precedence
	transformer := unicode.BOMOverride(enc.NewDecoder())
	out, _, err := transform.Bytes(transformer, data)
	if err != nil {
		return nil, false, fmt.Errorf("decode %s: %w", encName, err)
	}

	// If transform stripped BOM from UTF-8 or returned equal slice
	return out, !bytes.Equal(out, data), nil
}
