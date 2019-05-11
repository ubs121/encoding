package cdr

import (
	//"encoding/asn1"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
)

type GsmCdr struct {
	data []byte
}

const century = 20 * 100

func (f *GsmCdr) Convert() error {
	n := 0 // бичлэгийн тоо
	//var err error

	dat := f.data

	for len(dat) > 16 {
		r := new(GsmCdrRow)
		r.Length = int((dat[2]<<8)|dat[0]) + 2

		r.Version = dat[1]
		r.HotBilling = (dat[3] & 0x80) >> 7
		r.MscType = (dat[3] & 0x70) >> 4
		r.CallType = dat[3] & 0x0F
		r.Efficiency = dat[4] >> 4
		r.TerminationType = dat[4] & 0x0F
		r.TicketNumber = dat[5]
		r.CallOrigin = dat[6] >> 4
		r.ChargingIndicator = dat[6] & 0x0F
		r.Teleservice = dat[7]
		r.Bearerservice = dat[8]
		r.AllocDate = time.Date(century+bcd(dat[9]), time.Month(bcd(dat[10])), bcd(dat[11]), bcd(dat[12]), bcd(dat[13]), bcd(dat[14]), 0, time.Local)
		r.Duration = int((dat[15] << 16) | (dat[16] << 8) | dat[17])
		r.CallTimeStamp = time.Date(century+bcd(dat[9]), time.Month(bcd(dat[18])), bcd(dat[19]), bcd(dat[20]), bcd(dat[21]), bcd(dat[22]), 0, time.Local)

		rest := dat[23:] // эхний 23 байтыг алгасаж авах

		r.MobileStationID, rest = readTBCD(rest)
		r.SsData, rest = readSSBinary(rest)
		r.LinkInfo, rest = readISDN(rest)
		r.Subscriber, rest = readTBCD(rest)
		r.Mscid, rest = readISDN(rest)
		r.CallPartner, rest = readISDN(rest)

		// дуудлагыг ялгах
		if r.Teleservice == ShortMessageMT_OP {
			r.Type = "MO_SMS"
			r.CallType = r.Teleservice
			r.Subscriber = r.LinkInfo.Number
		} else if r.Teleservice == ShortMessageMT_PP {
			r.Type = "MT_SMS"
			r.CallType = r.Teleservice
		} else {
			switch r.CallType {
			case OrginatingCall:
			case OriginatingWithHOTBILL:
				r.Type = "MOC"
				r.Subscriber = r.LinkInfo.Number
				// дуудлага хийсэн байршил
				msLoclength := rest[0]
				if msLoclength != 0 {
					rest = rest[1:]
					r.LocationID.Mcc = int(rest[0]&0x0F)*100 + int(rest[1]&0x0F)*10 + int(rest[1]>>4)
					r.LocationID.Mnc = int(rest[2]&0x0F)*10 + int(rest[2]>>4)
					r.LocationID.Lac = int((rest[3] << 8) | rest[4])
					r.LocationID.Cell = int((rest[5] << 8) | rest[6])
				}
			case TerminatingCall:
				r.Type = "MTC"
				// дуудлага хийсэн байршил
				msLoclength := rest[0]
				if msLoclength != 0 {
					rest = rest[1:]
					r.LocationID.Mcc = int(rest[0]&0x0F)*100 + int(rest[1]&0x0F)*10 + int(rest[1]>>4)
					r.LocationID.Mnc = int(rest[2]&0x0F)*10 + int(rest[2]>>4)
					r.LocationID.Lac = int((rest[3] << 8) | rest[4])
					r.LocationID.Cell = int((rest[5] << 8) | rest[6])
				}
			case MSCForwardedTC, GMSCForwardedTC, MSCForwardTerminatingWithHOTBILL:
				r.Type = "MTC"
				// дуудлагын хамтрагч
				r.CallPartner, rest = readISDN(rest)
			case ReroutedForwarded:
				r.Type = "MOC"
				r.Subscriber = r.LinkInfo.Number
				// дуудлагын хамтрагч
				r.CallPartner, rest = readISDN(rest)
			case Handover:
				r.Type = "HANDOVER"
			}
		}

		// bearer
		bearerLen := rest[0]
		if bearerLen >= 6 {
			r.Bearer.ChannelType = (rest[0] >> 4)
			r.Bearer.SpeechVersion = (rest[0] & 0x0F)
			r.Bearer.Trunk = string(rest[1:6])
		}

		r.NetworkInfo, rest = readSSBinary(rest)

		// subscriber-аар ялгах
		if strings.HasPrefix(r.Subscriber, "42899") {
			bs, _ := json.Marshal(r)
			fmt.Println(string(bs))
			n++
		}

		dat = dat[r.Length:]

		if n > 10 {
			break
		}
	}

	fmt.Println(n, " calls parsed.")

	return nil
}

func (f *GsmCdr) Load(file string) error {
	var err error
	f.data, err = ioutil.ReadFile(file)
	return err
}

func (f *GsmCdr) SaveTo(file string) error {
	return nil
}

func bcdString(b []byte) (string, []byte) {
	length := b[0]
	b = b[1:]

	var s bytes.Buffer
	for i := byte(0); i < length; i++ {
		s.WriteByte((b[i] >> 4) + 48)
		if (b[i] & 0x0F) != 0x0F {
			s.WriteByte((b[i] & 0x0F) + 48)
		}
	}
	return s.String(), b[length:]
}

func bcd(val byte) int {
	return int(val>>4)*10 + int(val&0x0F)
}

func readSSBinary(b []byte) ([]byte, []byte) {
	length := (b[0] << 8) | b[1]
	b = b[2:]
	// TODO: SS  агуулгыг задалж унших
	return b[:length], b[length:]
}

func readISDN(b []byte) (ISDN, []byte) {
	var isdn ISDN

	length := b[0] - 1 // nature хасав
	isdn.Nature = b[1]
	b = b[2:]

	var number bytes.Buffer

	for i := byte(0); i < length; i++ {
		number.WriteByte((b[i] & 0x0F) + 48)

		if (b[i] & 0xF0) != 0xF0 {
			number.WriteByte((b[i] >> 4) + 48)
		}
	}
	isdn.Number = number.String()

	return isdn, b[length:]
}

func readTBCD(b []byte) (string, []byte) {
	length := b[0]
	b = b[1:]

	var s bytes.Buffer
	for i := byte(0); i < length; i++ {
		s.WriteByte((b[i] & 0x0F) + 48)

		if (b[i] & 0xF0) != 0xF0 {
			s.WriteByte((b[i] >> 4) + 48)
		}
	}
	return s.String(), b[length:]
}

// CallType
const (
	OrginatingCall                   = 0x00
	TerminatingCall                  = 0x01
	MSCForwardedTC                   = 0x02
	ReroutedTC                       = 0x04
	ReroutedForwarded                = 0x05
	GMSCForwardedTC                  = 0x06
	Handover                         = 0x0B
	OriginatingWithHOTBILL           = 0x0D
	TerminatingWithHOTBILL           = 0x0E
	MSCForwardTerminatingWithHOTBILL = 0x0F
)

// Teleservices
const (
	Speech                  = 0x10
	Telephone               = 0x11
	EmergencyCalls          = 0x12
	ShortMessageMT_PP       = 0x21
	ShortMessageMT_OP       = 0x22
	FacsimileGr3AlterSpeech = 0x61
	AutomaticFacsimileGr3   = 0x62
	NotUsed                 = 0xFF
)

// MSCTypes
const (
	MSC   = 0x00
	GMSC  = 0x01
	IWMSC = 0x03
)

// TerminationTypes
const (
	NormalClearing       = 0x01
	NetworkFailure       = 0x01
	LocalSystemFail      = 0x03
	RadioFail            = 0x04
	HandoverFail         = 0x05
	BlackIMEI            = 0x06
	CallDurationOverflow = 0x07
)

// CallOrigins
const (
	NationalCall            = 0x00
	InternationalCall       = 0x01
	ManualNationalCall      = 0x02
	ManualInternationalCall = 0x03
	IndicatorNotApplicable  = 0x0F
)

// ChargingIndicators
const (
	NoIndicationRecieved = 0x00
	Charge               = 0x01
	NoCharge             = 0x02
)

// SupplSerCode
const (
	IN          = 0x01
	OpInter     = 0x0D
	ForwardInfo = 0x0E
	Camel       = 0x02
	CallHold    = 0x03
	CUG         = 0x04
	AOC         = 0x05
	ECT         = 0x06
	MPTY_Master = 0x07
	MPTY_Slave  = 0x08
	DTMF        = 0x09
	Prefix      = 0xA0
	Dual        = 0x0C
	PUUS        = 0x10
	CLIP        = 0x11
)

// BearerServices
const (
	BS21 = 0x11 // 300 bit/s
	BS22 = 0x12 // 1200 bit/s
	BS24 = 0x14 // 2400 bit/s
	BS25 = 0x15 // 4800 bit/s
	BS26 = 0x16 // 9600 bit/s
	BS31 = 0x1A // 1200 bit/s
	BS32 = 0x1C // 2400 bit/s
	BS33 = 0x1D // 4800 bit/s
	BS34 = 0x1E // 9600 bit/s
)

type ISDN struct {
	Nature byte
	Number string
}

type Location struct {
	Mcc  int
	Mnc  int
	Lac  int
	Cell int
	// or
	Isdn ISDN
}

type BearerCapability struct {
	ChannelType   byte
	SpeechVersion byte
	Trunk         string
}

type GsmCdrRow struct {
	Length            int
	Type              string
	Version           byte
	HotBilling        byte
	MscType           byte
	CallType          byte
	Efficiency        byte
	TerminationType   byte
	TicketNumber      byte
	CallOrigin        byte
	ChargingIndicator byte
	Teleservice       byte
	Bearerservice     byte
	AllocDate         time.Time
	Duration          int
	CallTimeStamp     time.Time
	MobileStationID   string
	LinkInfo          ISDN
	Subscriber        string
	Mscid             ISDN
	CallPartner       ISDN
	LocationID        Location
	//LocationIDEx      string
	Bearer      BearerCapability
	NetworkInfo []byte
	SsData      []byte
}
