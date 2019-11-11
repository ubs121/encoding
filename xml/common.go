package xml


func _space(buf []byte) []byte {
	i := 0
	for i < len(buf) {
		if _, yes := spaceByte[buf[i]]; yes {
			i++
		} else {
			break
		}
	}
	return buf[i:]
}

func _skipTo(buf []byte, end byte) []byte {
	i := bytes.IndexByte(buf, end)
	if i < 0 {
		// not found, return empty slice
		return []byte{}
	}
	// 'i+1' skips the end byte
	return buf[i+1:]
}

func _readTo(buf []byte, end byte) ([]byte, []byte) {
	i := bytes.IndexByte(buf, end)
	if i < 0 {
		// not found
		return buf[:], []byte{}
	}

	// 'i+1' skips the end byte
	return buf[:i], buf[i+1:]
}

func _readName(buf []byte) ([]byte, []byte) {
	i := 0
	for i < len(buf) {
		if _, yes := nameByte[buf[i]]; yes {
			i++
		} else {
			break
		}
	}

	return buf[:i], buf[i:]
}

// Requirement: all must be in one line
func _readAttr(parent *Elem, buf []byte) []byte {

	for 0 < len(buf) {
		if buf[0] == '>' || buf[0] == '/' {
			// closing
			break
		}

		buf = _space(buf)

		at := new(Elem)
		at.Kind = Attribute
		at.name, buf = _readName(buf)

		buf = _space(buf)

		// value
		if buf[0] == '=' {
			buf = _space(buf[1:])

			// must be quote Example: <valid attribute = ">"/>
			if buf[0] == '"' {
				at.data, buf = _readTo(buf[1:], '"')
			} else {
				// error, but ignore, read till ' '
				at.data, buf = _readTo(buf, ' ')
			}
		} else {
			// by default
			// value = True ?
		}

		parent.Children = append(parent.Children, at)
	}

	return buf
}

func _readText(buf []byte) ([]byte, []byte) {
	i := 0
	for i < len(buf) {
		c := buf[i]

		if c == '<' {
			// end of text
			break
		} else if c == '"' { // quote
			// skip until closing quote,
			// including multi-lines
			// FIXME: what if non-closing quote
			j := bytes.IndexByte(buf[i+1:], '"')
			if j < 0 {
				// un-closed quote ?, read all ?
				i = len(buf)
				break
			}
			i += j + 2
		} else {
			// normal character
			i++
		}
	}

	return buf[:i], buf[i:]
}

var nameByte map[byte]bool
var spaceByte map[byte]bool

func initParser() {
	// init global data
	nameByte = make(map[byte]bool)
	_nameBytes := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ:0123456789_.-")
	for _, b := range _nameBytes {
		nameByte[b] = true
	}

	spaceByte = map[byte]bool{
		' ':  true,
		'\t': true,
		'\r': true,
		'\n': true,
	}

}