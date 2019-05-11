package cdr

import "time"

type CDRFile interface {
	// бинар өгөгдлийг санах ойд ачаална
	Load(file string) error

	// triple формат руу хувиргана
	Convert() error

	// triple өгөгдлийг файл руу хадгална
	SaveTo(file string) error
}

type CDRow struct {
	Type       string
	Date       time.Time
	Duration   int
	Subscriber string
	Serial     string
	//public string linkInfo;
	Class   byte
	Partner string
	Price   float32
	Lac     int
	Cell    int
}
