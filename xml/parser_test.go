package xml

import (
	"testing"
)

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

	testCase3 = `
		<doc>
		<abstract>Albedo () (, meaning 'whiteness') is the measure of the diffuse reflection of solar radiation out of the total solar radiation received by an astronomical body (e.g.</abstract>
		</doc>
		`
)

func TestParseChunk(t *testing.T) {
	initParser()

	chunk := new(Segment)
	chunk.data = []byte(testCase1)

	parseChunk(chunk)

	t.Log(chunk.children)

	got := len(chunk.children)
	expected := 1
	if got != expected {
		t.Errorf("len(chunk.children) = %d; want %d", got, expected)
	}

	root := chunk.children[0]
	got = len(root.children)
	expected = 8
	if got != expected {
		t.Errorf("root.children = %d; want %d", got, expected)
	}

}

func BenchmarkParseChunk(b *testing.B) {
	initParser()

	chunk := new(Segment)

	for n := 0; n < b.N; n++ {
		chunk.data = []byte(testCase1)
		parseChunk(chunk)
	}

}

func BenchmarkParseLines(b *testing.B) {
	initParser()

	chunk := new(Segment)

	for n := 0; n < b.N; n++ {
		chunk.data = []byte(testCase1)
		parseLines(chunk)
	}

}
