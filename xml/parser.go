// Copyright 2019 ubs121

// Package xml implements a concurrent, line by line parser
package xml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
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

// FastParser parses a XML file concurrently
// all XML elements must be on its own line
type FastParser struct {
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

// NewParser creates a new parser
// filename is a input XML file
// chanTrans is a subscriber channel for parsed outputs
func NewParser(filename string, chanTrans chan *Segment) *FastParser {
	cp := new(FastParser)
	cp.Concurrency = maxWorkers
	cp.ChunkSize = chunkSize
	cp.fileName = filename
	cp.chanParse = make(chan *Segment)
	cp.chanMerge = make(chan *Segment)
	cp.chanTransform = chanTrans

	return cp
}

// collect sub-trees & merge
func (cp *FastParser) merge() {
	start := time.Now()

	c := 0
	queueChunk := []*Segment{}
	currentPath := []*Segment{}
	expectedID := int64(0) // TODO: use real ID

	for {
		select {
		// parsed chunk
		case chunk := <-cp.chanMerge:
			// collect into the queue
			queueChunk = append(queueChunk, chunk)
			c++
		default:
			n := len(queueChunk)
			// TODO: check if it's an expected chunk
			chunk := queueChunk[n-1]
			queueChunk = queueChunk[:n-1]

			/* Merge chunks

			Chunk1: [ '(', '(', 'O','O','O' ]
			Chunk2: [ ')', '(', 'O','O','O' ]
			Chunk3: ['O', ')', ')' ]

			*/
			for _, el := range chunk.children {
				switch el.kind {
				case OpenTag:
					// push
					currentPath = append(currentPath, el)
					// TODO: add attributes as leaf
				case CloseTag:
					// pop
					// TODO: check node balance
					currentPath = currentPath[:len(currentPath)-1]

					// TODO: transform(el)
				case Node:
					// 'O' complete node
					if cp.chanTransform != nil {
						cp.chanTransform <- el
					}
				default:
					// TODO: what to do ???

				}
			}

			// release from memory
			//chunk.data = nil
			//chunk.result = nil

			// next expected chunk
			expectedID++

			if chunk.x < 0 {
				break
			}
		}

	}

	cp.mergeTime = time.Now().Sub(start)
}

/*  Non-strict, XML parser

Requirements:
 - chunk must contain at least one leaf
 - each tag must be on its own line
*/
func parseChunk(chunk *Segment) {

	buf := chunk.data
	n := len(buf)

	// expected number of elements, 120 characters per element
	//chunk.children = make([]*Segment, 0, n/120)
	stack := segmentStack(chunk.children)
	var tmpData []byte

	// TODO: count lines

	for 0 < len(buf) {
		// skip spaces
		buf = _space(buf)

		// text
		if 0 < len(buf) && buf[0] != '<' {

			tmpData, buf = _readText(buf)

			top := stack.Peek()
			if top != nil && top.kind == OpenTag {
				// concatenate on top.data
				top.data = append(top.data, tmpData...)
			} else {
				// parent-less chardata
				el := new(Segment)
				el.x = n - len(buf)
				el.kind = CharData
				stack.Push(el)
			}
		}

		// now, must be '<'
		if len(buf) == 0 || buf[0] != '<' {
			// error or EOF
			break
		}

		// buf[0] == '<'
		// buf[1] == name byte | '/' | '?' | '!' | *
		c := buf[1]

		if _, yes := nameByte[c]; yes { // start element
			el := new(Segment)
			el.x = n - len(buf)
			el.kind = OpenTag
			el.name, buf = _readName(buf[1:])

			if buf[0] == ' ' {
				// read attributes
				buf = _readAttr(el, buf)
			} else {
				//tmpAttr = nil
			}

			// self-closing tag?
			if buf[0] == '/' {
				buf = buf[1:]
				// TODO: close tag (leaf)
			}

			// must be '>'
			if buf[0] == '>' {
				buf = buf[1:]
			} else {
				// error !!! but ignore, skip till '>'
				buf = _skipTo(buf, '>')
			}

			// new element
			top := stack.Peek()
			if top != nil && top.kind == CloseTag {
				// link parent & child
				el.parent = top
				top.children = append(top.children, el)
			} else {
				// top-less
			}
			stack.Push(el)

		} else if c == '/' { // end element
			tmpData, buf = _readTo(buf[2:], '>')
			top := stack.Peek()
			if top != nil && top.kind == CloseTag {
				// change into a completed
				top.kind = Node

				// TODO: also check top.name ???
				if stack.Len() > 1 {
					stack.Pop()
				} else {
					// keep it in the stack,
					// because it's a top level element
				}
			} else {
				// top-less closing tag
				el := new(Segment)
				el.x = n - len(buf)
				el.kind = CloseTag
				el.name = tmpData

				stack.Push(el)
			}
		} else if c == '!' || c == '?' {
			// ignore, skip till '\n'
			buf = _skipTo(buf[2:], '\n')
		} else {
			// error: unrecognized element
			buf = _skipTo(buf[2:], '\n')
		}
	}

	chunk.children = stack

}

/*
Flow:
	- Main: split into chunks ->  Worker
	- Worker: build sub-tree (recognizing structure is first priority, so use line indention)
	- Worker: inform structure (un-closed nodes) -> TopTree
	- TopTree: update top-tree
	- Worker: ask top-tree <- TopTree
	- Worker: transform -> partial file  (subscriber)
	- Main: concatenate partial files
*/
// Run is a main routine
func (cp *FastParser) Run() error {
	startTime := time.Now()

	// init parser
	initParser()

	// open file
	file, err := os.Open(cp.fileName)
	checkError(err)
	defer file.Close()

	fi, err := file.Stat()
	checkError(err)

	cp.fileSize = fi.Size()

	cp.totalChunks = int(cp.fileSize / chunkSize)
	if cp.fileSize%chunkSize > 0 {
		cp.totalChunks++
	}

	// start file split
	go splitFile(file, cp.chanParse)

	var mutex = &sync.Mutex{}

	// TODO: how to terminate these workers?
	// start chunk parsers
	for i := 0; i < cp.Concurrency; i++ {
		go func() {
			for {
				// TODO: use signalling channel to terminate

				// read a raw chunk
				chunk := <-cp.chanParse

				start := time.Now()

				parseChunk(chunk)
				//parseLines(chunk)
				//parseNormal(chunk)

				mutex.Lock()
				cp.parseTime += time.Now().Sub(start)
				mutex.Unlock()

				// send to merger
				cp.chanMerge <- chunk
			}
		}()

	}

	// build XML tree
	cp.merge()
	// tree:=cp.merge()

	cp.totalTime = time.Now().Sub(startTime)

	// print parsing results
	fmt.Printf("Total chunks: %d\n", cp.totalChunks)
	fmt.Printf("Read time: %f sec\n", cp.readTime.Seconds())
	fmt.Printf("Parse time: %f sec (accumulated)\n", cp.parseTime.Seconds())
	fmt.Printf("Total time: %f sec\n", cp.totalTime.Seconds())

	// close channels
	close(cp.chanParse)
	close(cp.chanMerge)

	return nil
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
