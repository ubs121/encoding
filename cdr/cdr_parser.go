package main

import (
	"flag"
	"fmt"
	"memex/parser/cdr"
	"os"
)

func main() {
	var fileType = flag.String("type", "", "Файлын төрөл: GSM | SMS | IN | WLL")
	var fileName = flag.String("file", "", "Хөрвүүлэх файл")

	flag.Parse()

	if len(os.Args) < 2 {
		Usage()
		os.Exit(1)
	}

	var f cdr.CDRFile

	switch *fileType {
	case "GSM":
		f = new(cdr.GsmCdr)
		break
	case "IN":
		f = new(cdr.InCdr)
		break
	case "SMS":
		f = new(cdr.SmsCdr)
		break
	case "WLL":
		f = new(cdr.WllCdr)
		break
	default:
		fmt.Println("Not supported file type")
		flag.Usage()
		break
	}

	if f != nil {
		fmt.Printf("converting... %s\n", *fileName)

		f.Load(*fileName)
		f.Convert()
		f.SaveTo(*fileName + ".txt")

		fmt.Println("done.")
	}
}

func Usage() {
	fmt.Println("cdr\n")
	fmt.Println("Хэрэглэх заавар:")
	fmt.Println("  cdr <файлын нэр> [флаг]\n")
	fmt.Println("\nФлагууд:")
	flag.Parse()
	flag.PrintDefaults()
}
