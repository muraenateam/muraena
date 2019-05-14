package xmlquery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func findNode(root *Node, name string) *Node {
	node := root.FirstChild
	for {
		if node == nil || node.Data == name {
			break
		}
		node = node.NextSibling
	}
	return node
}

func childNodes(root *Node, name string) []*Node {
	var list []*Node
	node := root.FirstChild
	for {
		if node == nil {
			break
		}
		if node.Data == name {
			list = append(list, node)
		}
		node = node.NextSibling
	}
	return list
}

func testNode(t *testing.T, n *Node, expected string) {
	if n.Data != expected {
		t.Fatalf("expected node name is %s,but got %s", expected, n.Data)
	}
}

func testAttr(t *testing.T, n *Node, name, expected string) {
	for _, attr := range n.Attr {
		if attr.Name.Local == name && attr.Value == expected {
			return
		}
	}
	t.Fatalf("not found attribute %s in the node %s", name, n.Data)
}

func testValue(t *testing.T, val, expected string) {
	if val != expected {
		t.Fatalf("expected value is %s,but got %s", expected, val)
	}
}

func TestLoadURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := `<?xml version="1.0"?>
       <rss>
	   	<title></title>
	   </rss>`
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(s))
	}))
	defer server.Close()
	_, err := LoadURL(server.URL)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceURL(t *testing.T) {
	s := `
<?xml version="1.0"?>
<rss version="2.0" xmlns="http://www.example.com/" xmlns:dc="https://purl.org/dc/elements/1.1/">
<!-- author -->
<dc:creator><![CDATA[Richard Lawler]]></dc:creator>
<dc:identifier>21|22021348</dc:identifier>
</rss>
	`
	doc, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	top := FindOne(doc, "//rss")
	if top == nil {
		t.Fatal("rss feed invalid")
	}
	node := FindOne(top, "dc:creator")
	if node.Prefix != "dc" {
		t.Fatalf("expected node prefix name is dc but is=%s", node.Prefix)
	}
	if node.NamespaceURI != "https://purl.org/dc/elements/1.1/" {
		t.Fatalf("dc:creator != %s", node.NamespaceURI)
	}
	if strings.Index(top.InnerText(), "author") > 0 {
		t.Fatalf("InnerText() include comment node text")
	}
	if strings.Index(top.OutputXML(true), "author") == -1 {
		t.Fatal("OutputXML shoud include comment node,but not")
	}
}

func TestMultipleProcInst(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?>
<?xml-stylesheet type="text/xsl" media="screen" href="/~d/styles/rss2full.xsl"?>
<?xml-stylesheet type="text/css" media="screen" href="http://feeds.reuters.com/~d/styles/itemcontent.css"?>
<rss xmlns:feedburner="http://rssnamespace.org/feedburner/ext/1.0" version="2.0">
</rss>
	`
	doc, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}

	node := doc.FirstChild // <?xml ?>
	if node.Data != "xml" {
		t.Fatal("node.Data != xml")
	}
	node = node.NextSibling // New Line
	node = node.NextSibling // <?xml-stylesheet?>
	if node.Data != "xml-stylesheet" {
		t.Fatal("node.Data != xml-stylesheet")
	}
}

func TestParse(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?>
<bookstore>
<book>
  <title lang="en">Harry Potter</title>
  <price>29.99</price>
</book>
<book>
  <title lang="en">Learning XML</title>
  <price>39.95</price>
</book>
</bookstore>`
	root, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Error(err)
	}
	if root.Type != DocumentNode {
		t.Fatal("top node of tree is not DocumentNode")
	}

	declarNode := root.FirstChild
	if declarNode.Type != DeclarationNode {
		t.Fatal("first child node of tree is not DeclarationNode")
	}

	if declarNode.Attr[0].Name.Local != "version" && declarNode.Attr[0].Value != "1.0" {
		t.Fatal("version attribute not expected")
	}

	bookstore := root.LastChild
	if bookstore.Data != "bookstore" {
		t.Fatal("bookstore elem not found")
	}
	if bookstore.FirstChild.Data != "\n" {
		t.Fatal("first child node of bookstore is not empty node(\n)")
	}
	books := childNodes(bookstore, "book")
	if len(books) != 2 {
		t.Fatalf("expected book element count is 2, but got %d", len(books))
	}
	// first book element
	testNode(t, findNode(books[0], "title"), "title")
	testAttr(t, findNode(books[0], "title"), "lang", "en")
	testValue(t, findNode(books[0], "price").InnerText(), "29.99")
	testValue(t, findNode(books[0], "title").InnerText(), "Harry Potter")

	// second book element
	testNode(t, findNode(books[1], "title"), "title")
	testAttr(t, findNode(books[1], "title"), "lang", "en")
	testValue(t, findNode(books[1], "price").InnerText(), "39.95")

	testValue(t, books[0].OutputXML(true), `<book><title lang="en">Harry Potter</title><price>29.99</price></book>`)
}

func TestMissDeclaration(t *testing.T) {
	s := `<AAA>
		<BBB></BBB>
		<CCC></CCC>
	</AAA>`
	doc, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	node := FindOne(doc, "//AAA")
	if node == nil {
		t.Fatal("//AAA is nil")
	}
}

func TestMissingNamespace(t *testing.T) {
	s := `<root>
	<myns:child id="1">value 1</myns:child>
	<myns:child id="2">value 2</myns:child>
  </root>`
	_, err := Parse(strings.NewReader(s))
	if err == nil {
		t.Fatal("err is nil, want got invalid XML document")
	}
}

func TestTooNested(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?>
	<!-- comment here-->
    <AAA> 
        <BBB> 
            <DDD> 
                <CCC> 
                    <DDD/> 
                    <EEE/> 
                </CCC> 
            </DDD> 
        </BBB> 
        <CCC> 
            <DDD> 
                <EEE> 
                    <DDD> 
                        <FFF/> 
                    </DDD> 
                </EEE> 
            </DDD> 
        </CCC> 		
     </AAA>`
	root, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Error(err)
	}
	aaa := findNode(root, "AAA")
	if aaa == nil {
		t.Fatal("AAA node not exists")
	}
	ccc := aaa.LastChild
	if ccc.Data != "CCC" {
		t.Fatalf("expected node is CCC,but got %s", ccc.Data)
	}
	bbb := ccc.PrevSibling
	if bbb.Data != "BBB" {
		t.Fatalf("expected node is bbb,but got %s", bbb.Data)
	}
	ddd := findNode(bbb, "DDD")
	testNode(t, ddd, "DDD")
	testNode(t, ddd.LastChild, "CCC")
}

func TestSelectElement(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?>
    <AAA> 
        <BBB id="1"/>
        <CCC id="2"> 
            <DDD/>   
        </CCC> 
		<CCC id="3"> 
            <DDD/>
        </CCC> 
     </AAA>`
	root, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Error(err)
	}
	version := root.FirstChild.SelectAttr("version")
	if version != "1.0" {
		t.Fatal("version!=1.0")
	}
	aaa := findNode(root, "AAA")
	var n *Node
	n = aaa.SelectElement("BBB")
	if n == nil {
		t.Fatalf("n is nil")
	}
	n = aaa.SelectElement("CCC")
	if n == nil {
		t.Fatalf("n is nil")
	}

	var ns []*Node
	ns = aaa.SelectElements("CCC")
	if len(ns) != 2 {
		t.Fatalf("len(ns)!=2")
	}
}

func TestEscapeOutputValue(t *testing.T) {
	data := `<AAA>&lt;*&gt;</AAA>`

	root, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Error(err)
	}

	escapedInnerText := root.OutputXML(true)
	if !strings.Contains(escapedInnerText, "&lt;*&gt;") {
		t.Fatal("Inner Text has not been escaped")
	}

}
func TestOutputXMLWithNamespacePrefix(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?><S:Envelope xmlns:S="http://schemas.xmlsoap.org/soap/envelope/"><S:Body></S:Body></S:Envelope>`
	doc, _ := Parse(strings.NewReader(s))
	if s != doc.OutputXML(false) {
		t.Fatal("xml document missing some characters")
	}
}

func TestAttributeWithNamespace(t *testing.T) {
	s := `<?xml version="1.0" encoding="UTF-8"?><root xmlns:n1="http://www.w3.org">
   <good a="1" b="2" />
   <good a="1" n1:a="2" /></root>`
	doc, _ := Parse(strings.NewReader(s))
	n := FindOne(doc, "//good[@n1:a='2']")
	if n == nil {
		t.Fatal("n is nil")
	}
}

func TestOutputXMLWithCommentNode(t *testing.T) {
	s := `<?xml version="1.0" encoding="utf-8"?>
	<!-- Students grades are updated bi-monthly -->
	<class_list>
		<student>
			<name>Robert</name>
			<grade>A+</grade>
		</student>
	<!--
		<student>
			<name>Lenard</name>
			<grade>A-</grade>
		</student> 
	-->
	</class_list>`
	doc, _ := Parse(strings.NewReader(s))
	t.Log(doc.OutputXML(true))
	if e, g := "<!-- Students grades are updated bi-monthly -->", doc.OutputXML(true); strings.Index(g, e) == -1 {
		t.Fatal("missing some comment-node.")
	}
	n := FindOne(doc, "//class_list")
	t.Log(n.OutputXML(false))
	if e, g := "<name>Lenard</name>", n.OutputXML(false); strings.Index(g, e) == -1 {
		t.Fatal("missing some comment-node")
	}
}
