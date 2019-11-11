// Copyright 2019 ubs121

// Package xml implements a concurrent, non-strict parser
// It implements XML spec partially
/*
Flow:
	- Main: split into chunks ->  Worker
	- Worker: build sub-tree (recognizing structure is first priority, so use line indention)
	- Worker: inform structure (un-closed nodes) -> TopTree
	- TopTree: update top-tree
	- Worker: ask top-tree <- TopTree
	- Worker: transform -> partial file  (subscriber)
	- Main: concatenate partial files

	| Main | - chunks ->  | Worker1 | - subtree -> | Worker2 | ...*/
package xml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Total Memory  = maxWorkers * chunk size
const maxWorkers = 16              //
const chunkSize = 1024 * 1024 * 64 // 64Mb

// These are types of XML elements
const (
	Node      = iota // completed node (downward only), no parent ???
	OpenTag          // open tag
	CloseTag         // close tag
	CharData         // char data
	Attribute        // attribute
)

// Elem is a generic type for all kind of xml elements
type Elem struct {
	ID       int  // position
	Kind     byte // see const
	Name     []byte
	Parent   *Elem // parent, top element
	Children ElemList
	data     []byte
}

// Client is a subscriber that wants the parsed output
type Client struct {
	Query    []string
	Callback func(ElemList)
}

type rawData struct {
	parent    *rawData
	offset    int    // position in the raw file
	data      []byte // raw data
	parseTime time.Duration
	done      chan bool // signalling channel
	result    ElemList  // parsed result
}

// Parser parses a XML file concurrently
type Parser struct {
	// setup
	Concurrency int // number of workers, better to be relative to the number of runtime.NumCPU()
	ChunkSize   int // 1024 * 1024 * 64 // 64Mb

	// input
	fileName string
	fileSize int64
	rdr      io.Reader

	// channels
	chanParse chan *rawData // split -> || -> worker
	client    *Client       // worker -> || -> client

	// processing results (in seconds)
	totalChunks int
	readTime    time.Duration
	parseTime   time.Duration
	mergeTime   time.Duration
	totalTime   time.Duration
}

// NewParser creates a new parser
// filename - input XML file
// client - subscriber channel for the parsed outputs
func NewParser(filename string, client *Client) *Parser {
	cp := new(Parser)
	cp.Concurrency = maxWorkers
	cp.ChunkSize = chunkSize
	cp.fileName = filename
	cp.chanParse = make(chan *rawData)
	cp.client = client

	return cp
}

/*  Non-strict, XML parser

Requirements:
 - chunk must contain at least one leaf
*/
func parse(chunk *rawData) {
	start := time.Now()

	buf := chunk.data
	n := len(buf)

	// expected number of elements, 120 characters per element
	//chunk.Children = make([]*Elem, 0, n/120)
	stack := ElemList{}
	var tmpData []byte

	for 0 < len(buf) {
		// skip spaces
		buf = _space(buf)

		// text
		if 0 < len(buf) && buf[0] != '<' {

			tmpData, buf = _readText(buf)

			top := stack.peek()
			if top != nil && top.Kind == OpenTag {
				// concatenate on top.data
				top.data = append(top.data, tmpData...)
			} else {
				// parent-less chardata
				el := new(Elem)
				el.Kind = CharData
				stack.push(el)
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
			el := new(Elem)
			el.ID = n - len(buf)
			el.Kind = OpenTag
			el.Name, buf = _readName(buf[1:])

			if buf[0] == ' ' {
				// read attributes
				buf = _readAttr(el, buf)

				// TODO: assign IDs to attr
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
			top := stack.peek()
			if top != nil && top.Kind == CloseTag {
				// link parent & child
				el.Parent = top
				top.Children = append(top.Children, el)
			} else {
				// top-less
			}
			stack.push(el)

		} else if c == '/' { // end element
			tmpData, buf = _readTo(buf[2:], '>')
			top := stack.peek()
			if top != nil && top.Kind == CloseTag {
				// change into a complete node
				top.Kind = Node

				// TODO: check if top.name matches with tmpData ?
				if stack.len() > 1 {
					stack.pop()
				} else {
					// keep it in the stack,
					// because it's a top level element
					// expects the opening tag from upstream
				}
			} else {
				// top-less closing tag
				el := new(Elem)
				el.Kind = CloseTag
				el.Name = tmpData

				stack.push(el)
			}
		} else if c == '?' {
			// <? . * ?> processing instruction
			// ignore, skip till '\n'
			buf = _skipTo(buf[2:], '\n')
		} else if c == '!' {
			// <!-- .* --> comment
			// <![CDATA[ .* ]]> cdata

			// ignore, skip till '\n'
			buf = _skipTo(buf[2:], '\n') // FIXME: '>'
		} else {
			// error: unrecognized element
			buf = _skipTo(buf[2:], '\n') // FIXME: '>'
		}
	}

	chunk.result = stack
	chunk.parseTime += time.Now().Sub(start)
}

/* Worker routines */
func (cp *Parser) worker() {
	for {
		// read a raw data
		chunk := <-cp.chanParse

		// parse the chunk
		parse(chunk)

		// wait for top-worker
		if chunk.parent != nil {
			<-chunk.parent.done
		}

		// accumulate parsing time
		cp.parseTime += chunk.parseTime

		// TODO: walk tree & run matcher
		// TODO: call client.Callback(completedSubTree)
	}

}

// IDEA: use mmap
// splits the file and send to the parser
func (cp *Parser) splitFile() {
	var leftOver bytes.Buffer
	off := 0
	var lastChunk *rawData

	for {
		// read next chunk
		buf := make([]byte, cp.ChunkSize) // !new buffer allocation
		//n, err := io.ReadFull(cp.rdr, buf)
		n, err := cp.rdr.Read(buf)
		buf = buf[:n]

		// may have some data at the EOF
		if err == io.EOF {
			if n > 0 {
				// last chunk
				last := new(rawData)
				last.offset = off
				last.data = append(leftOver.Bytes(), buf[:n]...)
				last.parent = lastChunk

				cp.chanParse <- last

				lastChunk = last
			}

			break // sucessfull end
		}

		if err != nil {
			break // unknown error
		}

		// FIXME: how to cut when all inline, by '</' ?
		// cut by '\n', so don't half tag names
		i := bytes.LastIndexByte(buf, '\n')
		if i < 0 {
			err = errors.New("please format the XML file")
			i = n
		}

		chunk := new(rawData)
		chunk.offset = off
		chunk.data = append(leftOver.Bytes(), buf[:i]...)
		chunk.parent = lastChunk

		// send a chunk (it blocks until a worker read it)
		cp.chanParse <- chunk

		// keep left-over, excluding '\n'
		leftOver.Reset()
		leftOver.Write(buf[i+1:])

		// next
		off++
		lastChunk = chunk
	}

	// wait for last chunk
	<-lastChunk.done
}

// Run is a main routine
func (cp *Parser) Run() error {
	startTime := time.Now()

	// init parser
	initParser()

	// open file
	file, err := os.Open(cp.fileName)
	checkError(err)
	defer file.Close()

	fi, err := file.Stat()
	checkError(err)

	// calculate the number of chunks
	cp.fileSize = fi.Size()
	cp.totalChunks = int(cp.fileSize / chunkSize)
	if cp.fileSize%chunkSize > 0 {
		cp.totalChunks++
	}

	cp.rdr = file

	// start workers
	for i := 0; i < cp.Concurrency; i++ {
		go cp.worker()
	}

	// start file split
	cp.splitFile()

	cp.totalTime = time.Now().Sub(startTime)

	// close channels
	close(cp.chanParse)

	//cp.PrintStats()
	return nil
}

// PrintStats prints runtime statistics
func (cp *Parser) PrintStats() {
	fmt.Printf("Total chunks: %d\n", cp.totalChunks)
	fmt.Printf("Read time: %f sec\n", cp.readTime.Seconds())
	fmt.Printf("Parse time: %f sec (accumulated)\n", cp.parseTime.Seconds())
	fmt.Printf("Total time: %f sec\n", cp.totalTime.Seconds())
}


// ElemList is a list of Elem
type ElemList []*Elem

func (stack ElemList) isEmpty() bool {
	return len(stack) == 0
}

func (stack ElemList) len() int {
	return len(stack)
}

func (stack *ElemList) peek() *Elem {
	if stack.isEmpty() {
		return nil
	}
	s := *stack
	n := len(s)
	return s[n-1]
}

func (stack *ElemList) push(el *Elem) {
	*stack = append(*stack, el)
}

func (stack *ElemList) pop() *Elem {
	s := *stack
	n := len(s)
	top := s[n-1]
	*stack = s[0 : n-1]
	return top
}

func makeCopy(b []byte) []byte {
	b1 := make([]byte, len(b))
	copy(b1, b)
	return b1
}

func checkError(e error) {
	if e != nil {
		panic(e)
	}
}
