package xml

import "encoding/xml"

type LineParser struct {
    rdr io.Reader
    data []byte
}

func NewLineParser(r io.Reader) *LineParser {
    pr:=&LineParser {
        rdr : r
    }
    initParser()

	return pr
}

/*  Non-strict, XML parser

Requirements:
 - chunk must contain at least one leaf
*/
func (pr *LineParser) Token() (xml.Token, error) {
	line := pr.data

    // chomp spaces
    line = _space(line)

    var tmpData []byte
    // text
    if line[0] != '<' {
        tmpData, line = _readText(line)
        return xml.CharData(tmpData), nil
    }

    // line[0] == '<'
    // line[1] == name byte | '/' | '?' | '!' | *
    c := line[1]

    if _, yes := nameByte[c]; yes { // start element
        el := new(Elem)
        el.ID = n - len(line)
        el.Kind = OpenTag
        el.Name, line = _readName(line[1:])

        if line[0] == ' ' {
            // read attributes
            line = _readAttr(el, line)

            // TODO: assign IDs to attr
        } else {
            //tmpAttr = nil
        }

        // self-closing tag?
        if line[0] == '/' {
            line = line[1:]
            // TODO: close tag (leaf)
        }

        // must be '>'
        if line[0] == '>' {
            line = line[1:]
        } else {
            // error !!! but ignore, skip till '>'
            line = _skipTo(line, '>')
        }


    } else if c == '/' { // end element
        tmpData, line = _readTo(line[2:], '>')
        return xml.EndElement(tmpData), nil
    } else if c == '?' {
        // <? . * ?> processing instruction
        return nil, nil
    } else if c == '!' {
        // <!-- .* --> comment
        // <![CDATA[ .* ]]> cdata
        return nil, nil
    }

    // error: unrecognized element
    return nil, errors.New("unrecognized element")
}