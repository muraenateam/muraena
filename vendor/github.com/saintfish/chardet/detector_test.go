package chardet_test

import (
	"github.com/saintfish/chardet"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDetector(t *testing.T) {
	type file_charset_language struct {
		File     string
		IsHtml   bool
		Charset  string
		Language string
	}
	var data = []file_charset_language{
		{"utf8.html", true, "UTF-8", ""},
		{"utf8_bom.html", true, "UTF-8", ""},
		{"8859_1_en.html", true, "ISO-8859-1", "en"},
		{"8859_1_da.html", true, "ISO-8859-1", "da"},
		{"8859_1_de.html", true, "ISO-8859-1", "de"},
		{"8859_1_es.html", true, "ISO-8859-1", "es"},
		{"8859_1_fr.html", true, "ISO-8859-1", "fr"},
		{"8859_1_pt.html", true, "ISO-8859-1", "pt"},
		{"shift_jis.html", true, "Shift_JIS", "ja"},
		{"gb18030.html", true, "GB-18030", "zh"},
		{"euc_jp.html", true, "EUC-JP", "ja"},
		{"euc_kr.html", true, "EUC-KR", "ko"},
		{"big5.html", true, "Big5", "zh"},
	}

	textDetector := chardet.NewTextDetector()
	htmlDetector := chardet.NewHtmlDetector()
	buffer := make([]byte, 32<<10)
	for _, d := range data {
		f, err := os.Open(filepath.Join("testdata", d.File))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		size, _ := io.ReadFull(f, buffer)
		input := buffer[:size]
		var detector = textDetector
		if d.IsHtml {
			detector = htmlDetector
		}
		result, err := detector.DetectBest(input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Charset != d.Charset {
			t.Errorf("Expected charset %s, actual %s", d.Charset, result.Charset)
		}
		if result.Language != d.Language {
			t.Errorf("Expected language %s, actual %s", d.Language, result.Language)
		}
	}
}
