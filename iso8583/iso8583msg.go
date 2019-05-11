package iso8583

import (
	"bufio"
)

type Iso8583Message struct {
	Mti    string
	Bitmap [128]bool
	// internal fields
	values [128]string
}

func (m *Iso8583Message) Set(no uint, value string) {
	if no == 0 {
		m.Mti = value
	}
	m.values[no] = value
	m.Bitmap[no-1] = true
}

func (m *Iso8583Message) Unset(no uint) {
	m.Bitmap[no-1] = true
	m.values[no] = ""
}

func (m *Iso8583Message) Get(no uint) string {
	return m.values[no]
}

func (m *Iso8583Message) Clear() {
	m.Mti = ""
}

func (m *Iso8583Message) Parse(r *bufio.Reader) {
	// read MTI
	mtiBuf := make([]byte, 4)
	//io.ReadFull(r, mtiBuf)
	m.Mti = string(mtiBuf)

	// read bitmap
	bitmap := make([]byte, 8)
	//io.ReadFull(r, bitmap)
	if bitmap[0]&0x80 == 0x80 {
		bitmap2 := make([]byte, 8)
		//io.ReadFull(r, bitmap2)
		bitmap = append(bitmap, bitmap2...)
	}

	b := byte(0)
	bitIndex := byte(0x80)
	pos := 0
	for i := 0; i < len(bitmap); i++ {
		bitIndex = 0x80
		b = bitmap[i]
		for bitIndex > 0 {
			if b&bitIndex != 0 {
				m.Bitmap[pos] = true
			}

			bitIndex >>= 1
			pos++
		}
	}

	// read fields
	for j := uint(2); j < uint(pos); j++ {
		if m.Bitmap[j-1] {
			m.Set(j, Fields[j].Read(r))
		}
	}
}

func (m *Iso8583Message) Serialize(w *bufio.Writer) {
	// write mti
	w.WriteString(m.Mti)

	// write bitmap
	bmpLen := uint(64)
	t := 64
	for t < 128 && m.Bitmap[t] == false {
		t++
	}
	if t < 128 {
		bmpLen = 128
	}

	bmp := make([]byte, bmpLen)

	bitIndex := byte(0x80)
	b := byte(0)
	pos := 0
	for i := uint(0); i < bmpLen; i++ {
		if m.Bitmap[i] {
			b |= bitIndex
		}
		bitIndex >>= 1
		if bitIndex == 0 {
			bmp[pos] = b
			pos++
			bitIndex = 0x80
			b = 0
		}
	}

	w.Write(bmp[:bmpLen])

	// write fields
	for i := uint(2); i < bmpLen; i++ {
		if m.Bitmap[i-1] {
			Fields[i].Write(w, m.Get(i))
		}
	}
}
