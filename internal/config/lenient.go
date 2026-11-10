package config

import "bytes"

// Lenient rewrites the loose JSON used by many subscriptions into JSON
// encoding/json accepts: line and block comments outside strings, trailing
// commas, and raw control characters inside strings. Slashes that belong
// to URLs stay, because the scan tracks string state.
func Lenient(raw []byte) []byte {
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var out bytes.Buffer
	out.Grow(len(raw))
	inString := false
	escaped := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escaped {
				escaped = false
				switch c {
				case '\n':
					out.WriteString(`\n`)
				case '\r':
					out.WriteString(`\r`)
				case '\t':
					out.WriteString(`\t`)
				default:
					if c >= 0x20 {
						out.WriteByte('\\')
						out.WriteByte(c)
					}
				}
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				out.WriteByte(c)
				inString = false
			case '\n':
				out.WriteString(`\n`)
			case '\r':
				out.WriteString(`\r`)
			case '\t':
				out.WriteString(`\t`)
			default:
				if c >= 0x20 {
					out.WriteByte(c)
				}
			}
			continue
		}
		if c == '"' {
			out.WriteByte(c)
			inString = true
			continue
		}
		if c == '/' && i+1 < len(raw) {
			if raw[i+1] == '/' {
				i += 2
				for i < len(raw) && raw[i] != '\n' {
					i++
				}
				continue
			}
			if raw[i+1] == '*' {
				i += 2
				for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
					i++
				}
				if i+1 < len(raw) {
					i++
				}
				continue
			}
		}
		if c == ',' {
			j := i + 1
			for j < len(raw) && (raw[j] == ' ' || raw[j] == '\n' || raw[j] == '\r' || raw[j] == '\t') {
				j++
			}
			if j < len(raw) && (raw[j] == '}' || raw[j] == ']') {
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}
