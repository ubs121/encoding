// Copyright 2015 ubs121

// Package iso8583 implements a fast ISO 8583 decoder
package iso8583

import (
	"bufio"
	"bytes"
)

type IField interface {
	Read(r *bufio.Reader) string
	Write(w *bufio.Writer, s string)
}

var Fields [128]IField

type Field struct {
	Type   string
	Length int
}

type LLField struct {
	Type   string
	Length int
}

type LLLField struct {
	Type   string
	Length int
}

func (f *Field) Read(r *bufio.Reader) string {
	buf := make([]byte, f.Length)
	//io.ReadFull(r, buf)
	if f.Type[0] == 'n' {
		i := 0
		for i < len(buf) && buf[i] == '0' {
			i++
		}
		return string(buf[i:])
	}

	return string(bytes.Trim(buf, " "))
}

func (f *LLField) Read(r *bufio.Reader) string {
	lenbuf := make([]byte, 2)
	r.Read(lenbuf)
	vlen := int(lenbuf[0] - '0')
	vlen = vlen*10 + int(lenbuf[1]-'0')

	buf := make([]byte, vlen)
	r.Read(buf)
	return string(buf)
}

func (f *LLLField) Read(r *bufio.Reader) string {
	lenbuf := make([]byte, 3)
	r.Read(lenbuf)
	vlen := int(lenbuf[0] - '0')
	vlen = vlen*10 + int(lenbuf[1]-'0')
	vlen = vlen*10 + int(lenbuf[2]-'0')

	buf := make([]byte, vlen)
	r.Read(buf)
	return string(buf)
}

// Write functions
func (f *Field) Write(w *bufio.Writer, s string) {
	if len(s) > f.Length {
		buf := make([]byte, f.Length)
		for i := 0; i < f.Length; i++ {
			buf[i] = s[i]
		}
		w.Write(buf)
	} else {
		pad := byte(' ')
		if f.Type[0] == 'n' {
			pad = '0'
		}
		buf := make([]byte, f.Length-len(s))
		for i := 0; i < f.Length-len(s); i++ {
			buf[i] = pad
		}
		w.Write(buf)
		w.WriteString(s)
	}
}

func (f *LLField) Write(w *bufio.Writer, s string) {
	l := len(s)

	lenbuf := make([]byte, 2)
	// length header
	lenbuf[0] = byte(l / 10)
	lenbuf[1] = byte(l % 10)
	// data
	w.Write(lenbuf)
	w.WriteString(s)
}

func (f *LLLField) Write(w *bufio.Writer, s string) {
	l := len(s)

	lenbuf := make([]byte, 3)
	// length header
	lenbuf[0] = byte(l / 100)
	lenbuf[1] = byte((l / 100) / 10)
	lenbuf[2] = byte((l / 100) % 10)
	// data
	w.Write(lenbuf)
	w.WriteString(s)
}

func InitFieldTypes() {
	Fields[CardNo] = &LLField{"s..", 19}
	Fields[ProcCode] = &Field{"n", 6}
	Fields[AMOUNT] = &Field{"n", 12}
	Fields[TrxDate] = &Field{"n", 10}
	Fields[TraceNo] = &Field{"n", 6}
	Fields[LocalTime] = &Field{"n", 6}
	Fields[LocalDate] = &Field{"n", 4}
	Fields[15] = &Field{"n", 4}
	Fields[18] = &Field{"n", 4}
	Fields[22] = &Field{"n", 3}
	Fields[25] = &Field{"n", 2}
	Fields[26] = &Field{"n", 2}
	Fields[32] = &LLField{"n..", 11}
	Fields[33] = &LLField{"n..", 11}
	Fields[35] = &LLField{"n..", 37}
	Fields[36] = &LLLField{"ans...", 104}
	Fields[37] = &Field{"s", 12}
	Fields[38] = &Field{"s", 6}
	Fields[ResponseCode] = &Field{"s", 2}
	Fields[41] = &Field{"s", 8}
	Fields[42] = &Field{"s", 15}
	Fields[43] = &Field{"s", 40}
	Fields[44] = &LLLField{"ans..", 25}
	Fields[48] = &LLLField{"ans...", 999}
	Fields[CURRENCY] = &Field{"s", 3}
	Fields[52] = &Field{"n", 16}
	Fields[53] = &Field{"n", 16}
	Fields[54] = &LLLField{"ans...", 120}
	Fields[60] = &LLLField{"ans...", 999}
	Fields[61] = &LLLField{"ans...", 999}
	Fields[62] = &LLLField{"ans...", 999}
	Fields[63] = &LLLField{"ans...", 999}
	Fields[90] = &Field{"n", 42}
	Fields[95] = &Field{"s", 42}
	Fields[Account1] = &LLField{"ans..", 30}
	Fields[Account2] = &LLField{"ans..", 30}
}

// Field numbers
const (
	MTI    = 0
	Bitmap = 1
	CardNo = 2
	// Processing Code
	ProcCode = 3
	// Transaction Amount
	AMOUNT = 4
	// Settlement Amount
	SettleAmount = 5
	// Transmission Date and Time
	TrxDate = 7
	// Conversion Rate Settlement
	_009_CONVERSION_RATE_SETTLEMENT = 9
	// Track 1 Data
	DATA1 = 45
	// Security Related Control Information
	_053_SECURITY_RELATED_CONTROL_INFORMATION = 53
	// Authorisation Life Cycle
	_057_AUTHORISATION_LIFE_CYCLE = 57
	// Authorising Agent Institution
	_058_AUTHORISING_AGENT_INSTITUTION = 58
	// Systems Trace Audit Number
	TraceNo = 11
	// Field 12 - Time, Local Transaction
	LocalTime = 12
	// Field 13 - Date, Local Transaction
	LocalDate = 13
	// Field 14 - Date, Expiration
	ExpireDate = 14
	// Field 15 - Date, Settlement
	SettleDate = 15
	// Field 16 - Date, Conversion
	ConversionDate = 16
	// Field 18 - Merchant Type
	MCC = 18
	// Field 22 - POS Entry Mode
	PosEntry = 22
	// Field 23 - Card Sequence Number
	_023_CARD_SEQUENCE_NUM = 23
	// Field 25 - POS Condition Code
	PosType = 25
	// Field 26 - POS PIN Capture Code
	PinCapture = 26
	// Authorisation ID Response
	AuthIDResponse = 27
	// Transaction fee amount
	TranFeeAmount = 28
	// Settlement fee amount
	SettlementFeeAmount = 29
	// Transaction processing fee amount
	_030_TRAN_PROC_FEE_AMOUNT = 30
	// Settlement processing fee amount
	_031_SETTLEMENT_PROC_FEE_AMOUNT = 31
	// Field 32 - Acquiring Institution ID Code
	ACQUIRER = 32
	// Field 33 - Forwarding Institution ID Code
	FORWARDER = 33
	// Field 35 - Track 2 Data
	DATA2 = 35
	// Field 37 - Retrieval Reference Number
	RefNo = 37
	// Field 38 - Authorization ID Response
	ApprovalCode = 38
	// Field 39 - Response Code
	ResponseCode = 39
	// Field 40 - Service Restriction Code
	ServiceRestrictionCode = 40
	// Field 41 - Card Acceptor Terminal ID
	TERMINAL = 41
	// Field 42 - Card Acceptor ID Code
	BRANCH = 42
	// Field 43 - Card Acceptor Name Location
	DESC = 43
	// Field 44 - Additional Response Data
	_044_ADDITIONAL_RESPONSE_DATA = 44
	// Field 48 - Additional Data
	AdditionalData = 48
	// Field 49 - Currency Code, Transaction
	CURRENCY = 49
	// Field 50 - Currency Code, Settlement
	SettleCurrency = 50
	// Field 52 - PIN Data
	PinData = 52
	// Field 54 - Additional Amounts
	AdditionalAmount = 54
	// Field 56 - Message Reason Code
	_056_MESSAGE_REASON_CODE = 56
	// Settlement Code
	SettleCode = 66
	// Extended Payment Code
	_067_EXTENDED_PAYMENT_CODE = 67
	// Network Management Information Code
	_070_NETWORK_MANAGEMENT_INFORMATION_CODE = 70
	// Date Action
	_073_DATE_ACTION = 73
	// Credits, Number
	_074_CREDITS_NUMBER = 74
	// Credits Reversal, Number
	_075_CREDITS_REVERSAL_NUMBER = 75
	// Debits, Number
	_076_DEBITS_NUMBER = 76
	// Debits Reversal, Number
	_077_DEBITS_REVERSAL_NUMBER = 77
	// Transfers, Number
	TRANSFER_NUMBER = 78
	// Transfers Reversal, Number
	_079_TRANSFER_REVERSAL_NUMBER = 79
	// Inquiries, Number
	_080_INQUIRIES_NUMBER = 80
	// Authorisations, Number
	_081_AUTHORISATIONS_NUMBER = 81
	// Credits, Processing Fee Amount
	_082_CREDITS_PROCESSING_FEE_AMOUNT = 82
	// Credits, Transaction Fee Amount
	_083_CREDITS_TRANSACTION_FEE_AMOUNT = 83
	// Debits, Processing Fee Amount
	_084_DEBITS_PROCESSING_FEE_AMOUNT = 84
	// Debits, Transaction Fee Amount
	_085_DEBITS_TRANSACTION_FEE_AMOUNT = 85
	// Credits, Amount
	CreditsAmount = 86
	// Credits Reversal, Amount
	_087_CREDITS_REVERSAL_AMOUNT = 87
	// Debits, Amount
	DebitsAmount = 88
	// Debits Reversal, Amount
	_089_DEBITS_REVERSAL_AMOUNT = 89
	// Original Data Elements
	_090_ORIGINAL_DATA_ELEMENTS = 90
	// File Update Code
	FileUpdateCode = 91
	// Replacement Amounts
	_095_REPLACEMENT_AMOUNTS = 95
	// Amount Net Settlement
	_097_AMOUNT_NET_SETTLEMENT = 97
	// Payee
	PAYEE = 98
	// Field 100 - Receiving Institution ID Code
	RECEIVER = 100
	// Field 101 - File Name
	FileName = 101
	// Field 102 - Account Identification 1
	Account1 = 102
	// Field 103 - Account Identification 2
	Account2 = 103
	// Payments, Number
	PaymentsNumber = 118
	// Payments Reversal, Number
	PaymentsReversalNumber = 119
)
