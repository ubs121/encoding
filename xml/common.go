package xml

import (
	"bytes"
	"errors"
	"io"
	"time"
)

// Total Memory  = maxWorkers * chunk size
const maxWorkers = 16              //
const chunkSize = 1024 * 1024 * 64 // 64Mb

// These are types of XML elements
const (
	File      = iota // whole file
	Chunk            // data chunk
	Node             // completed node (downward only), no parent ???
	OpenTag          // open tag
	CloseTag         // close tag
	CharData         // char data
	Attribute        // attribute
)

// Segment is a generic type for all kind of xml elements
// i.e broken xml nodes
// also message between parallel routines
type Segment struct {
	kind     byte // see const
	name     []byte
	data     []byte
	children []*Segment
	parent   *Segment // parent, top element
	x        int      // vertical position
	y        int      // horizontal position
}

type chanElems chan *Segment
type segmentStack []*Segment

var nameByte map[byte]bool
var spaceByte map[byte]bool

// Parser parses a XML file concurrently
// all XML elements must be on its own line
type Parser struct {
	// setup
	Concurrency int // number of workers, better to be relative to the number of CPUs, runtime.NumCPU()
	ChunkSize   int // 1024 * 1024 * 64 // 64Mb

	// input
	fileName string
	fileSize int64
	rdr      io.Reader

	// channels
	chanParse chan *Segment // splitFile -> || -> parser
	chanMerge chan *Segment // parser -> || -> merge

	// processing results (in seconds)
	totalChunks int
	readTime    time.Duration
	parseTime   time.Duration
	mergeTime   time.Duration
	totalTime   time.Duration

	// output stream
	chanTransform chan *Segment
}

// NewFastParser creates a new parser
// filename is a input XML file
// chanTrans is a subscriber channel for parsed outputs
func NewFastParser(filename string, chanTrans chan *Segment) *FastParser {
	cp := new(FastParser)
	cp.Concurrency = maxWorkers
	cp.ChunkSize = chunkSize
	cp.fileName = filename
	cp.chanParse = make(chan *Segment)
	cp.chanMerge = make(chan *Segment)
	cp.chanTransform = chanTrans

	return cp
}

// NewLineParser creates a new parser
// filename is a input XML file
// chanTrans is a subscriber channel for parsed outputs
func NewLineParser(filename string, chanTrans chan *Segment) *LineParser {
	cp := new(LineParser)
	cp.Concurrency = maxWorkers
	cp.ChunkSize = chunkSize
	cp.fileName = filename
	cp.chanParse = make(chan *Segment)
	cp.chanMerge = make(chan *Segment)
	cp.chanTransform = chanTrans

	return cp
}

// splits the file and send to parser
func splitFile(rdr io.Reader, out chan *Segment) {
	var leftOver bytes.Buffer
	var parent *Segment

	nChunk := 0

	for {
		// read next chunk
		buf := make([]byte, chunkSize) // new buffer allocation
		//n, err := io.ReadFull(cp.rdr, buf)
		n, err := rdr.Read(buf)
		buf = buf[:n]

		// may have some data at the EOF
		if err == io.EOF {
			if n > 0 {
				// last chunk
				last := new(Segment)
				last.x = nChunk
				last.kind = Chunk
				last.data = append(leftOver.Bytes(), buf[:n]...)
				last.parent = parent

				out <- last

				nChunk++
			}

			break // sucessfull end
		}

		if err != nil {
			break // unknown error
		}

		// cut by '\n', so don't half tag names
		i := bytes.LastIndexByte(buf, '\n')
		if i < 0 {
			err = errors.New("please format the XML file")
			break
		}

		chunk := new(Segment)
		chunk.x = nChunk
		chunk.kind = Chunk
		chunk.data = append(leftOver.Bytes(), buf[:i]...)
		chunk.parent = parent

		// send a chunk (it blocks until a worker read it)
		out <- chunk

		// keep left-over, excluding '\n'
		leftOver.Reset()
		leftOver.Write(buf[i+1:])

		// next chunk ID
		nChunk++

	}
}

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
func _readAttr(parent *Segment, buf []byte) []byte {

	for 0 < len(buf) {
		if buf[0] == '>' || buf[0] == '/' {
			// closing
			break
		}

		buf = _space(buf)

		at := new(Segment)
		at.kind = Attribute
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

		parent.children = append(parent.children, at)
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

func (stack segmentStack) IsEmpty() bool {
	return len(stack) == 0
}

func (stack segmentStack) Len() int {
	return len(stack)
}

func (stack *segmentStack) Peek() *Segment {
	if stack.IsEmpty() {
		return nil
	}
	s := *stack
	n := len(s)
	return s[n-1]
}

func (stack *segmentStack) Push(el *Segment) {
	*stack = append(*stack, el)
}

func (stack *segmentStack) Pop() *Segment {
	s := *stack
	n := len(s)
	top := s[n-1]
	*stack = s[0 : n-1]
	return top
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}
