package analyzer

import (
	"testing"
)

const testHTML = `
<html><body>
<div class="txt-list">
  <ul>
    <li><a href="/book/1">Book One</a><span class="author">Author A</span></li>
    <li><a href="/book/2">Book Two</a><span class="author">Author B</span></li>
  </ul>
</div>
<div class="v-list-item">
  <div class="v-title">Novel Title</div>
  <div class="v-author">Author C</div>
  <img src="/cover.jpg" />
</div>
<div class="novel-item">
  <a href="/book/3">Book Three</a>
</div>
<div id="booklist">
  <table><tbody>
    <tr><td>Row1</td></tr>
    <tr><td>Row2</td></tr>
  </tbody></table>
</div>
</body></html>
`

func TestCssSelectorClassText(t *testing.T) {
	// .v-list-item should return text of matching elements (Fix 2+3)
	results := cssGetStringList(".v-list-item", testHTML, "")
	if len(results) == 0 {
		t.Fatal("Expected results for .v-list-item, got empty")
	}
	found := false
	for _, r := range results {
		if r == "Novel Title Author C" || len(r) > 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected text content for .v-list-item, got %v", results)
	}
}

func TestCssSelectorTagLi(t *testing.T) {
	// class.txt-list.0@tag.li — the @tag.li part should NOT be misinterpreted as attribute "li"
	results := cssGetStringList("class.txt-list.0@tag.li", testHTML, "")
	if len(results) == 0 {
		t.Fatal("Expected results for class.txt-list.0@tag.li, got empty")
	}
	t.Logf("tag.li results: %v", results)
	for _, r := range results {
		if r == "" {
			t.Error("Expected non-empty text for each <li>")
		}
	}
}

func TestCssSelectorHref(t *testing.T) {
	// class.txt-list.0@tag.a.0@href — should extract href attribute
	results := cssGetStringList("class.txt-list.0@tag.a.0@href", testHTML, "")
	if len(results) == 0 {
		t.Fatal("Expected href results, got empty")
	}
	if results[0] != "/book/1" {
		t.Errorf("Expected /book/1, got %s", results[0])
	}
}

func TestCssSelectorImgSrc(t *testing.T) {
	// .v-list-item@img@src — extract img src
	results := cssGetStringList(".v-list-item@img@src", testHTML, "")
	if len(results) == 0 {
		t.Fatal("Expected src results, got empty")
	}
	if results[0] != "/cover.jpg" {
		t.Errorf("Expected /cover.jpg, got %s", results[0])
	}
}

func TestCssSelectorNovelCell(t *testing.T) {
	// class.novel-item should return text content
	results := cssGetStringList("class.novel-item", testHTML, "")
	if len(results) == 0 {
		t.Fatal("Expected results for class.novel-item, got empty")
	}
	t.Logf("novel-item results: %v", results)
}

func TestCssSelectorById(t *testing.T) {
	// #booklist@tag.tr — find tr elements inside #booklist
	results := cssGetStringList("#booklist@tag.tr", testHTML, "")
	if len(results) < 2 {
		t.Fatalf("Expected at least 2 tr results, got %d: %v", len(results), results)
	}
}
