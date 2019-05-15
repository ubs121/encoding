package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

func parseLines(chunk *Segment) {
	buf := chunk.data

	lines := bytes.Split(buf, []byte{'\n'}) // all must be sub-slices
	tags := make([]Segment, len(lines))     // destination array

	// TODO: calculate hierarchy from indents

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
}

func parseNormal(chunk *Segment) {
	dec := xml.NewDecoder(bytes.NewReader(chunk.data))
	dec.Strict = false

	count := 0

	for {
		// next token
		t, err := dec.RawToken()
		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Printf("parseNormal() failed with '%s'\n", err)
			continue // ignore
			//break
		}

		switch v := t.(type) {
		case xml.StartElement:
			count++
		case xml.EndElement:
			// TODO: check balance ???
			// if stack[l-1] == v.Name.Local
		case xml.CharData:
			v = bytes.TrimSpace(v)
		case xml.Comment:
			// comment
		case xml.ProcInst:
			// processing instruction like <?xml version="1.0"?>

		case xml.Directive:
			// directive like <!text>

		}

	}
}
