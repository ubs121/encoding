package xml

import (
	"testing"
)

// Test cases: https://www.w3.org/XML/Test/xmlconf-20020606.xml
var (
	testCase1 = `
		<book id="bk101"   isbn = "12243433444">
			<author>Gambardella, Matthew</author>
			<title>XML Developer's Guide</title>
			<genre>Computer</genre>
			<price>44.95</price>
			<publish_date>2000-10-01</publish_date>
			<description>An in-depth look at creating applications 
			with XML.</description>
		</book>`

	testCase2 = `
		<rule name="rule1">
			<valid expr="$1 < 10"/>
			<valid expr="$1 < 20"> 
			     <apply>This is a "$1 > 10 and $1 < 20 " case </apply>
			</valid>
		</rule>`

	testCase3broken = `
		doc>
		<doc>
		<abstract>Albedo () (, meaning 'whiteness') is the measure of the diffuse reflection of solar radiation out of the total solar radiation received by an astronomical body (e.g.</abstract>
		</doc>
		`
)

func TestParse(t *testing.T) {
	pr:=NewLineParser()
	dec:=xml.NewTokenDecoder(pr)

	for {
		t, err:=dec.Token()
		if err == io.EOF {
			break
		}

		switch v:=t.(type) {
		case xml.StartElement:
			// TODO: assert
		case xml.EndElement:
			// TODO: assert
		case xml.CharData:
			// TODO: assert
		}
	}
}


func BenchmarkParseChunk(b *testing.B) {
	initParser()

	chunk := new(rawData)

	for n := 0; n < b.N; n++ {
		chunk.data = []byte(testCase1)
		parse(chunk)
	}

}

