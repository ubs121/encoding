package iso8583

import (
	"bufio"
	"fmt"
	"testing"
	"time"
)

type NullOut struct {
}

func (no *NullOut) Write(p []byte) (n int, err error) {
	return 0, nil
}

func TestPerf(t *testing.T) {
	print("Iso8583 write test ...")

	// init
	InitFieldTypes()

	loops := int64(1000000)

	buf := bufio.NewWriter(new(NullOut))
	//buf:=bytes.NewBuffer(make([]byte, 1000000))

	start := time.Now()

	msg := new(Iso8583Message)

	for i := loops; i > 0; i-- {
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

	end := time.Now()

	fmt.Println("\nResult: ", loops*1e9/end.Sub(start).Nanoseconds())
}
