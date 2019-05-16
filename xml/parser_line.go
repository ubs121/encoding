package xml

import (
	"bytes"
)

func parseLines(chunk *Segment) {
	buf := chunk.data

	lines := bytes.Split(buf, []byte{'\n'}) // all must be sub-slices
	tags := make([]Segment, len(lines))     // meta information

	// TODO: calculate hierarchy from indents

	// TODO: inform structure (all of them ???) to a top-tree worker
	// [')', ')', 'O', 'O', ')', '(', 'O' ]

	// TODO: do actual parsing ('O' nodes)
	for i, line := range lines {
		// TrimSpace returns a subslice of s by slicing off all leading and
		// trailing white space, as defined by Unicode.
		line = bytes.TrimSpace(line)

		if len(line) == 0 {
			// skip
			continue
		}

		if line[0] == '<' {
			if _, yes := nameByte[line[1]]; yes {
				// start tag
				tags[i].kind = OpenTag
				tags[i].name, line = _readName(line[1:])

				if line[0] == ' ' {
					// read attributes
					line = _readAttr(&tags[i], line)
				} else {
					//tmpAttr = nil
				}

				// self-closing tag?
				if line[0] == '/' {
					// TODO: close tag (leaf)
					tags[i].kind = Node // complete tag
					line = line[1:]
				}

				// must be '>'
				if line[0] == '>' {
					// proper ending
					line = line[1:]
				} else {
					// error !!! but ignore, skip till '>'
					buf = _skipTo(buf, '>')
				}
			} else if line[1] == '/' {
				// end tag
				tags[i].kind = CloseTag
				tags[i].name = line[2 : len(line)-1] // exclude '>'
			} else if line[1] == '?' {
				// skip
			} else if line[1] == '!' {
				// skip
			} else {
				// error !!!
			}
		} else {
			// char data
			tags[i].kind = CharData
			tags[i].data = line
		}

	}

	// TODO: wait for top-tree
	// TODO: transform
}
