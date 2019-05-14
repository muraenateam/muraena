package xmlquery

import (
	"strings"
	"testing"
)

// https://msdn.microsoft.com/en-us/library/ms762271(v=vs.85).aspx
const xmlDoc = `
    <?xml version="1.0"?>
<catalog>
   <!-- book list-->
   <book id="bk101">
      <author>Gambardella, Matthew</author>
      <title>XML Developer's Guide</title>
      <genre>Computer</genre>
      <price>44.95</price>
      <publish_date>2000-10-01</publish_date>
      <description>An in-depth look at creating applications 
      with XML.</description>
   </book>
   <book id="bk102">
      <author>Ralls, Kim</author>
      <title>Midnight Rain</title>
      <genre>Fantasy</genre>
      <price>5.95</price>
      <publish_date>2000-12-16</publish_date>
      <description>A former architect battles corporate zombies, 
      an evil sorceress, and her own childhood to become queen 
      of the world.</description>
   </book>
   <book id="bk103">
      <author>Corets, Eva</author>
      <title>Maeve Ascendant</title>
      <genre>Fantasy</genre>
      <price>5.95</price>
      <publish_date>2000-11-17</publish_date>
      <description>After the collapse of a nanotechnology 
      society in England, the young survivors lay the 
      foundation for a new society.</description>
   </book>
</catalog>`

var doc = loadXML(xmlDoc)

func TestXPath(t *testing.T) {
	if list := Find(doc, "//book"); len(list) != 3 {
		t.Fatal("count(//book) != 3")
	}
	if node := FindOne(doc, "//book[@id='bk101']"); node == nil {
		t.Fatal("//book[@id='bk101] is not found")
	}
	if node := FindOne(doc, "//book[price>=44.95]"); node == nil {
		t.Fatal("//book/price>=44.95 is not found")
	}
	if list := Find(doc, "//book[genre='Fantasy']"); len(list) != 2 {
		t.Fatal("//book[genre='Fantasy'] items count is not equal 2")
	}
	var c int
	FindEach(doc, "//book", func(i int, n *Node) {
		c++
	})
	l := len(Find(doc, "//book"))
	if c != l {
		t.Fatal("count(//book) != 3")
	}
	c = 0
	FindEachWithBreak(doc, "//book", func(i int, n *Node) bool {
		if c == l - 1 {
			return false
		}
		c++
		return true
	})
	if c != l - 1 {
		t.Fatal("FindEachWithBreak failed to stop.")
	}
	node := FindOne(doc, "//book[1]")
	if node.SelectAttr("id") != "bk101" {
		t.Fatal("//book[1]/@id != bk101")
	}
}

func TestXPathCdUp(t *testing.T) {
	doc := loadXML(`<a><b attr="1"/></a>`)
	node := FindOne(doc, "/a/b/@attr/..")
	t.Logf("node = %#v", node)
	if node == nil || node.Data != "b" {
		t.Fatal("//b/@id/.. != <b/>")
	}
}

func TestNavigator(t *testing.T) {
	nav := &NodeNavigator{curr: doc, root: doc, attr: -1}
	nav.MoveToChild() // New Line
	nav.MoveToNext()
	nav.MoveToNext() // catalog
	if nav.curr.Data != "catalog" {
		t.Fatal("current node name != `catalog`")
	}
	nav.MoveToChild() // New Line
	nav.MoveToNext()  // comment node
	if nav.curr.Type != CommentNode {
		t.Fatal("node type not CommentNode")
	}
	nav.Value()
	nav.MoveToNext() // New Line
	nav.MoveToNext() //book
	nav.MoveToChild()
	nav.MoveToNext() // book/author
	if nav.LocalName() != "author" {
		t.Fatalf("node error")
	}
	nav.MoveToParent() // book
	nav.MoveToNext()   // next book
	if nav.curr.SelectAttr("id") != "bk102" {
		t.Fatal("node error")
	}
}

func loadXML(s string) *Node {
	node, err := Parse(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	return node
}
