package robotstxt

import (
	//    "os"
	"strconv"
	"testing"
)

func TestScan001(t *testing.T) {
	sc := newByteScanner("test-001", false)
	if _, err := sc.Scan(); err == nil {
		t.Fatal("Empty ByteScanner should fail on Scan.")
	}
}

func TestScan002(t *testing.T) {
	sc := newByteScanner("test-002", false)
	sc.Feed([]byte("foo"), true)
	_, err := sc.Scan()
	//print("---", tok, err)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestScan004(t *testing.T) {
	sc := newByteScanner("test-004", false)
	sc.Feed([]byte("\u2010"), true)
	_, err := sc.Scan()
	//println("---", tok, err)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func TestScan005(t *testing.T) {
	sc := newByteScanner("test-005", true)
	sc.Feed([]byte("\xd9\xd9"), true)
	_, err := sc.Scan()
	//println("---", tok, err)
	if err != nil {
		t.Fatal(err.Error())
	}
	if sc.ErrorCount != 2 {
		t.Fatal("Expecting ErrorCount be exactly 2.")
	}
}

func TestScan006(t *testing.T) {
	sc := newByteScanner("test-006", false)
	s := "# comment \r\nSomething: Somewhere\r\n"
	sc.Feed([]byte(s), true)
	tokens, err := sc.ScanAll()
	if err != nil {
		t.Fatal(err.Error())
	}
	//println("--- len(tokens):", len(tokens))
	if len(tokens) != 4 {
		t.Fatal("Expecting exactly 4 tokens.")
	}
	if tokens[0] != "\n" || tokens[1] != "Something" || tokens[2] != "Somewhere" || tokens[3] != "\n" {
		t.Fatal("Wrong tokens read:", strconv.Quote(tokens[0]), strconv.Quote(tokens[1]), strconv.Quote(tokens[2]), strconv.Quote(tokens[3]))
	}
}

func TestScan007(t *testing.T) {
	sc := newByteScanner("test-007", false)
	s := "# comment \r\n# more comments\n\nDisallow:\r"
	sc.Feed([]byte(s), true)
	tokens, err := sc.ScanAll()
	if err != nil {
		t.Fatal(err.Error())
	}
	//println("--- len(tokens):", len(tokens))
	if len(tokens) != 4 {
		t.Fatal("Expecting exactly 4 tokens.")
	}
	if tokens[0] != "\n" || tokens[1] != "\n" || tokens[2] != "Disallow" || tokens[3] != "\n" {
		t.Fatal("Wrong tokens read:", strconv.Quote(tokens[0]), strconv.Quote(tokens[1]), strconv.Quote(tokens[2]), strconv.Quote(tokens[3]))
	}
}

func TestScanUnicode8BOM(t *testing.T) {
	sc := newByteScanner("test-bom", false)
	sc.Feed([]byte(robotsTextVanityfair), true)
	tokens, err := sc.ScanAll()
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(tokens) == 0 {
		t.Fatal("Read zero tokens.")
	}
	if tokens[0] != "User-agent" {
		t.Fatal("Expecting first token: User-agent")
	}
}
