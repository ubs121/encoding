// Copyright 2019 ubs121

// Package xml implements a concurrent, line by line parser
package xml

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// FastParser parses a XML file concurrently
// all XML elements must be on its own line
type FastParser Parser

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
