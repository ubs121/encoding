package iso8583

import (
	"bufio"
	"testing"
)

type NullOut struct {
}

func (no *NullOut) Write(p []byte) (n int, err error) {
	return 0, nil
}

func BenchmarkSerialize(b *testing.B) {
	// init
	InitFieldTypes()

	buf := bufio.NewWriter(new(NullOut))
	//buf:=bytes.NewBuffer(make([]byte, 1000000))

	msg := new(Iso8583Message)

	for i := 0; i < b.N; i++ {
		msg.Mti = "0200"
		msg.Set(2, "3125")
		msg.Set(7, "0104132431")
		msg.Set(11, "1")
		msg.Set(12, "132431")
		msg.Set(13, "0104")
		msg.Set(37, "1762745214")
		msg.Set(39, "00")
		msg.Set(48, "01000abcdefghijkl                    ")

		msg.Serialize(buf)
	}
}
