package xml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Parser is a non-strict, concurrent xml parser
type Parser struct {
	file        *os.File
	concurrency int
	client      chan chan []Elem
	leaves      map[string]string // used schema
	shortest    []int
	err         error
}

// NewParser creates a parser
func NewParser(file *os.File, leaves map[string]string) *Parser {
	pr := &Parser{}
	pr.file = file
	pr.leaves = leaves
	pr.concurrency = concurrency

	initParser()

	// TODO: parse the filter
	// TODO: decide concurrency based on file size

	chanClient := make(chan chan []Elem, concurrency)
	go func() {
		//splitMmap(file, chanClient)
		pr.split(file, chanClient)

		close(chanClient)
	}()

	return pr

}

// parse xml, normal file read
func (pr *Parser) split(file *os.File, client chan chan []Elem) {
	msgNo := 0
	cutSize := chunkSize
	off := 0

	var leftOver bytes.Buffer
	var wg sync.WaitGroup

	for {
		if msgNo == 0 {
			cutSize = 1024 // first chunk, let it be small
		} else {
			cutSize = chunkSize
		}

		// read next chunk
		buf := make([]byte, cutSize) // !new buffer allocation
		n, err := file.Read(buf)
		buf = buf[:n]

		// may have some data at the EOF
		if err == io.EOF {
			if n > 0 {
				// last chunk
				chOut := make(chan []Elem)
				wg.Add(1)
				go func(buf []byte, out chan []Elem) {
					defer wg.Done()

					// TODO: pass filter
					d := newDecoder(buf, nil)
					elems := d.parse()

					if d.err != nil {
						// TODO: error handling
					}

					out <- elems[:]
					close(out)
				}(append(leftOver.Bytes(), buf[:n]...), chOut)

				client <- chOut
			}

			break // sucessfull end
		}

		if err != nil {
			break // unknown error
		}

		// cut by '</tag>', so don't half tag names
		i := indexLastEndTag(buf)
		if i < 0 {
			// reason: root end should be in the last cut
			panic("invalid xml file")
		}

		chOut := make(chan []Elem)
		wg.Add(1)
		go func(buf []byte, out chan []Elem) {
			defer wg.Done()

			// TODO: pass filter
			d := newDecoder(buf, nil)
			elems := d.parse()

			out <- elems[:] // elems != elems[:]
			close(out)
		}(append(leftOver.Bytes(), buf[:i]...), chOut)

		// wait
		client <- chOut

		// keep left-over
		leftOver.Reset()
		leftOver.Write(buf[i:])

		// next
		msgNo++
		off += n
	}
	wg.Wait()
}

// xml chunk decoder
type decoder struct {
	buf    []byte            // xml chunk
	off    int               // chunk offset in xml file
	n      int               // chunk length
	p      int               // current parsing position
	line   int               // current line
	stk    stack             // parsing stack
	filter map[string]string // filter
	skip   bool              // skip state
	err    error             // decoding error
}

func newDecoder(b []byte, filter map[string]string) *decoder {
	d := new(decoder)
	d.buf = b
	d.n = len(b)
	d.p = 0
	d.filter = filter
	return d
}

// parse a chunk of xml
func (d *decoder) parse() []Elem {
	elems := []Elem{}

	for !d.eof() {
		d.space()

		if d.eof() {
			break
		}

		if d.buf[d.p] != '<' {
			txt := d.text()
			if !d.skip {
				elems = append(elems, CharData(txt))
			}
		}

		if d.buf[d.p] != '<' {
			panic(fmt.Errorf("expected '<', got %c", d.buf[d.p]))
		}

		d.p++           // skip '<'
		c := d.buf[d.p] // what's after '<' ?

		if nameByte[c] > 0 { // start tag
			name := d.name()

			if !d.match(name) {
				// skip by text search
				d.skipElement(name)
				continue
			}

			var attr []byte
			//var attr [][]byte
			if d.buf[d.p] == ' ' {
				// read attributes
				// attr = d.attr()
				// if len(attr) > 0 {

				// }
				d.p++ // skip ' '
				a := bytes.IndexByte(d.buf[d.p:], '>')
				if a < 0 {
					panic("expected '>'")
				}
				attr = d.buf[d.p : d.p+a]
				d.p += a
			}

			// empty element ?
			empty := false
			if d.buf[d.p] == '/' {
				empty = true
				d.p++
			}

			// must be '>'
			if d.buf[d.p] == '>' {
				d.p++
			} else {
				panic("expected '>' after start tag ")
			}

			// push
			if !empty {
				//d.push(name)
			}

			elems = append(elems, StartTag{string(name), attr, 0})
		} else if c == '/' { // end tag
			d.p++
			name := d.readEnd()

			d.pop()

			// TODO: add only if incomplete end
			elems = append(elems, EndTag(name))
		} else if c == '?' {
			// <? . * ?> parsing instruction
			d.skipTo('<')
		} else if c == '!' {
			// <!-- .* --> comment
			// <![CDATA[ .* ]]> cdata
			d.skipTo('<')
		} else {
			// other data starts with '<'
			d.skipTo('<')
		}

		assert(d.stk.size() < 50, "xml depth is unusual")
	}

	// TODO: parse open tags

	return elems

}

// checks if it's a interested tag
func (d *decoder) match(tag []byte) bool {
	return true
}

func (d *decoder) eof() bool {
	return d.p >= d.n
}

// skip spaces
func (d *decoder) space() {
	for d.p < d.n && spaceByte[d.buf[d.p]] > 0 {
		d.p++
	}
}

// read name (nameByte)*(\s|>)
func (d *decoder) name() []byte {
	i := d.p
	for nameByte[d.buf[i]] > 0 { // d.p < d.n &&
		i++
	}
	name := d.buf[d.p:i]
	d.p = i
	return name
}

// chardata: (.*)<
// ampersand (&) and left angle bracket (<) must not appear in their literal form
func (d *decoder) text() []byte {
	i := bytes.IndexByte(d.buf[d.p:], '<')
	if i < 0 {
		d.err = errors.New("text() failed: can't find trailing '<' ")
		return d.buf[d.p:] // return all
	}
	txt := d.buf[d.p : d.p+i]
	d.p += i
	return txt
}

// skip until 'c'
func (d *decoder) skipTo(c byte) {
	i := bytes.IndexByte(d.buf[d.p:], c)
	if i < 0 {
		d.err = fmt.Errorf("skipTo() failed for %c", c)
		return
	}
	d.p += i
}

// skip whole element
func (d *decoder) skipElement(name []byte) {
	// set skip state
	d.skip = true

	bal := 0 // check tag balance, 0 means balanced
	m := len(name)

	for d.p < d.n {
		i := bytes.Index(d.buf[d.p:], name)

		if i < 0 {
			// no end tag
			break
		}

		d.p += i // position of match

		if i == 0 {
			// byte sequence with same name, so skip
		} else if d.buf[d.p-1] == '<' {
			// start tag with same name
			bal--
		} else if d.buf[d.p-1] == '/' { // promising, could be a match

			if d.p > 1 && d.buf[d.p-2] == '<' {
				bal++

				if bal == 0 { // match found
					// end of skip
					d.skip = false
					// update offset
					d.p += m + 1 // +1 is for '>'
					return
				}
			}

		}

		d.p += m
	}
	// no end tag found within the chunk (buf)
	return
}

// read till 'c'
func (d *decoder) readTo(c byte) []byte {
	i := bytes.IndexByte(d.buf[d.p:], c)
	if i < 0 {
		return d.buf[d.p:]
	}
	txt := d.buf[d.p : d.p+i]
	d.p += i
	return txt
}

// read end tag </tag>
func (d *decoder) readEnd() []byte {
	i := d.p
	for nameByte[d.buf[i]] > 0 { // d.p < d.n &&
		i++
	}

	name := d.buf[d.p:i]

	// skip till '>'
	for i < d.n && d.buf[i] != '>' {
		i++
	}

	d.p = i + 1
	return name
}

// attribute: (name=value)*
// Well-formedness constraint: No < in Attribute Values
// returns ret=[name1, value1, name2, value2,... ]
func (d *decoder) attr() [][]byte {

	var ret [][]byte
	var name, val []byte
	for {
		d.space()

		if d.eof() || d.buf[d.p] == '>' || d.buf[d.p] == '/' {
			// end of attributes
			break
		}

		// name
		name = d.name()
		ret = append(ret, name)

		// space
		d.space()

		// value
		if d.buf[d.p] == '=' {
			d.p++
			d.space()

			// must be quote Example: <valid attribute = ">"/>
			if d.buf[d.p] == '"' {
				d.p++
				val = d.readTo('"')
				d.p++ // skip '"'
			} else {
				d.space()
				val = d.readTo(' ')
				d.p++ // skip ' '
			}

			ret = append(ret, val)
		} else {
			// empty, set default value ?
			// Note: also could pop this attribute
			ret = append(ret, []byte{})

		}
	}

	return ret
}

// open elements, parsing state
type stack []Elem

func (d *decoder) push(tag Elem) {
	d.stk = append(d.stk, tag)
}

func (d *decoder) pop() Elem {
	el, stk := d.stk[d.stk.size()-1], d.stk[:d.stk.size()-1]
	d.stk = stk
	return el
}

func (d *decoder) peek() Elem {
	return d.stk[d.stk.size()-1]
}

// stack size
func (stk stack) size() int {
	return len(stk)
}

// empty ?
func (stk stack) empty() bool {
	return len(stk) == 0
}
