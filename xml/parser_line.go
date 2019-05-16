package xml

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"
)

/*
Line::
  x // line no 
  y // indent
  parent // parent.x < x
  children // range = [x+1: end]

Steps:
	- Main: split into chunks ->  Worker
	- Worker: build sub-tree (recognizing structure is first priority, so use line indention)
	- Worker: inform structure (un-closed nodes) -> TopTree
	- TopTree: update top-tree
	- Worker: ask top-tree <- TopTree
	- Worker: transform -> partial file  (subscriber)
	- Main: concatenate partial files
*/

// LineParser parses line by line
// all XML elements must be on its own line
type LineParser Parser

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

// Run is a main routine
func (lp *LineParser) Run() error {
	startTime := time.Now()

	// init parser
	initParser()

	// open file
	file, err := os.Open(lp.fileName)
	checkError(err)
	defer file.Close()

	fi, err := file.Stat()
	checkError(err)

	lp.fileSize = fi.Size()

	lp.totalChunks = int(lp.fileSize / chunkSize)
	if lp.fileSize%chunkSize > 0 {
		lp.totalChunks++
	}

	// start file split
	go splitFile(file, lp.chanParse)

	var mutex = &sync.Mutex{}

	// TODO: how to terminate these workers?
	// start chunk parsers
	for i := 0; i < lp.Concurrency; i++ {
		go func() {
			for {
				// TODO: use signalling channel to terminate

				// read a raw chunk
				chunk := <-lp.chanParse

				start := time.Now()

				parseLines(chunk)
				//parseLines(chunk)
				//parseNormal(chunk)

				mutex.Lock()
				lp.parseTime += time.Now().Sub(start)
				mutex.Unlock()

				// send to merger
				lp.chanMerge <- chunk
			}
		}()

	}

	// build XML tree
	//lp.merge()
	// tree:=lp.merge()

	lp.totalTime = time.Now().Sub(startTime)

	// print parsing results
	fmt.Printf("Total chunks: %d\n", lp.totalChunks)
	fmt.Printf("Read time: %f sec\n", lp.readTime.Seconds())
	fmt.Printf("Parse time: %f sec (accumulated)\n", lp.parseTime.Seconds())
	fmt.Printf("Total time: %f sec\n", lp.totalTime.Seconds())

	// close channels
	close(lp.chanParse)
	close(lp.chanMerge)

	return nil
}
