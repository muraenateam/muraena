package chardet_test

import (
	"fmt"
	"github.com/saintfish/chardet"
)

var (
	zh_gb18030_text = []byte{
		71, 111, 202, 199, 71, 111, 111, 103, 108, 101, 233, 95, 176, 108, 181, 196, 210, 187, 214, 214, 190, 142, 215, 103, 208, 205, 163, 172, 129, 75, 176, 108,
		208, 205, 163, 172, 178, 162, 190, 223, 211, 208, 192, 172, 187, 248, 187, 216, 202, 213, 185, 166, 196, 220, 181, 196, 177, 224, 179, 204, 211, 239, 209, 212,
		161, 163, 10,
	}
)

func ExampleTextDetector() {
	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(zh_gb18030_text)
	if err == nil {
		fmt.Printf(
			"Detected charset is %s, language is %s",
			result.Charset,
			result.Language)
	}
	// Output:
	// Detected charset is GB-18030, language is zh
}
