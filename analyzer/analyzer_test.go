package analyzer

import (
	"testing"

	"github.com/fastschema/qjs"
)

// ============================================================
// CSS / JSoup Tests
// ============================================================

const testHTMLFull = `<html><body>
<div class="book-like">
<a href="/book/1"><h4>斗破苍穹</h4><span>天蚕土豆</span><img src="/c/1.jpg"/></a>
<a href="/book/2"><h4>武动乾坤</h4><span>天蚕土豆</span><img src="/c/2.jpg"/></a>
</div>
<div class="txt-list">
  <ul>
    <li><a href="/b/1">First</a><span class="s4">A1</span></li>
    <li><a href="/b/2">Second</a><span class="s4">A2</span></li>
    <li><a href="/b/3">Third</a><span class="s4">A3</span></li>
  </ul>
</div>
<div class="mod block book-all-list">
  <div class="bd">
    <li><div class="right">
      <a class="name" href="/n/1/">苍穹</a>
      <span class="author">土豆</span>
    </div></li>
    <li><div class="right">
      <a class="name" href="/n/2/">乾坤</a>
      <span class="author">土豆</span>
    </div></li>
  </div>
</div>
<section class="grid-item">
  <a href="/g/1"><h4>GridBook1</h4></a>
</section>
<section class="grid-item">
  <a href="/g/2"><h4>GridBook2</h4></a>
</section>
<div class="info">作者：天蚕土豆 | 玄幻 | 322万字</div>
</body></html>`

func TestExtractHTML_ReturnsMultipleItems(t *testing.T) {
	// extractHTML should iterate .Each() to return individual elements
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	items := ar.GetElements(".book-like@a")
	if len(items) != 2 {
		t.Fatalf("Expected 2 items from .book-like@a, got %d", len(items))
	}
}

func TestExtractHTML_TagSelector(t *testing.T) {
	// Bare tag name "a" should be treated as a selector, not extraction instruction
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	items := ar.GetElements("a")
	if len(items) < 4 {
		t.Fatalf("Expected >=4 <a> elements, got %d", len(items))
	}
}

func TestMultiClassSelector(t *testing.T) {
	// "class.mod block book-all-list" should become ".mod.block.book-all-list"
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	items := ar.GetElements("class.mod block book-all-list@class.bd@tag.li@class.right")
	if len(items) != 2 {
		t.Fatalf("Expected 2 items from multi-class selector, got %d", len(items))
	}
	ia := NewAnalyzeRule(items[0], "https://test.com", pool)
	name := ia.GetString("class.name@text")
	if name != "苍穹" {
		t.Errorf("Expected name=苍穹, got %q", name)
	}
}

func TestExcludeModifier(t *testing.T) {
	// "li!0" should exclude the first <li>
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	items := ar.GetElements(".txt-list@tag.ul@tag.li!0")
	if len(items) != 2 {
		t.Fatalf("Expected 2 items (3 li minus index 0), got %d", len(items))
	}
}

func TestBareTagWithIndex(t *testing.T) {
	// "a.0@href" should find <a> tags and extract index 0's href
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	href := ar.GetString("a.0@href")
	if href != "/book/1" {
		t.Errorf("Expected /book/1, got %q", href)
	}
}

func TestOrCascade_CSS(t *testing.T) {
	// "h5@text||h4@text" should try h5 first, fall back to h4
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	name := ar.GetString("h5@text||h4@text")
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹 via || cascade, got %q", name)
	}
}

func TestOrCascade_Elements(t *testing.T) {
	// ".book-like a||.grid-item" should try first, fall back to second
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	items := ar.GetElements(".nonexistent||.grid-item")
	if len(items) != 2 {
		t.Fatalf("Expected 2 grid-items via || cascade, got %d", len(items))
	}
}

func TestHashPostProcessing(t *testing.T) {
	// "class.info@ownText##作者：" should extract ownText then remove "作者："
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	result := ar.GetString("class.info@ownText##作者：")
	if result == "" {
		t.Fatal("Expected non-empty result")
	}
	// "作者：" should be removed
	if len(result) >= 6 && result[:6] == "作者：" {
		t.Errorf("Expected '作者：' to be stripped, got %q", result)
	}
	t.Logf("Hash post-processing result: %q", result)
}

func TestOrCascade_WithHashPostProcessing(t *testing.T) {
	// "class.author@text||class.info@ownText##作者：" should cascade then post-process
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	result := ar.GetString("class.author@text||class.info@ownText##作者：")
	if result == "" {
		t.Fatal("Expected non-empty result from || with ##")
	}
	t.Logf("|| + ## result: %q", result)
}

// ============================================================
// {{...}} Template Tests
// ============================================================

func TestInlineTemplate_CSSExpr(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	name := ar.GetString(`{{@h4@text||h3@text||""}}{{$.name||""}}`)
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹 from template, got %q", name)
	}
}

func TestInlineTemplate_OrCascadeInside(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testHTMLFull, "https://test.com", pool)
	name := ar.GetString(`{{@h3@text||h4@text||""}}`)
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹 from || inside template, got %q", name)
	}
}

// ============================================================
// JSON Tests
// ============================================================

const testJSON = `[{"name":"斗破苍穹","author":"天蚕土豆","tags":"玄幻","category":"小说","status":"完结","update":"2024-01-01","word":"322万"},{"name":"武动乾坤","author":"天蚕土豆","tags":"玄幻","category":"小说","status":"完结","update":"2024-02-01","word":"280万"}]`

func TestJsonPath_GetElements(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testJSON, "https://test.com", pool)
	items := ar.GetElements("$[*]")
	if len(items) != 2 {
		t.Fatalf("Expected 2 JSON items from $[*], got %d", len(items))
	}
}

func TestJsonPath_FieldExtraction(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(testJSON, "https://test.com", pool)
	items := ar.GetElements("$[*]")
	ia := NewAnalyzeRule(items[0], "https://test.com", pool)
	name := ia.GetString(`{{@h4@text||h3@text||""}}{{$.name||""}}`)
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹 from JSON, got %q", name)
	}
	author := ia.GetString(`{{@span@text||""}}{{$.author||""}}`)
	if author != "天蚕土豆" {
		t.Errorf("Expected 天蚕土豆 from JSON, got %q", author)
	}
}

func TestAndConcat_JSON(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	ar := NewAnalyzeRule(`{"tags":"玄幻","category":"小说","status":"完结","update":"2024-01-01"}`, "https://test.com", pool)
	kind := ar.GetString("$.tags&&$.category&&$.status&&$.update")
	if kind == "" {
		t.Fatal("Expected non-empty && concatenation")
	}
	if kind != "玄幻 小说 完结 2024-01-01" {
		t.Errorf("Expected '玄幻 小说 完结 2024-01-01', got %q", kind)
	}
}

// ============================================================
// JS Stubs Tests
// ============================================================

func TestStub_CookieDefined(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	rt, err := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Put(rt)
	ctx := rt.Context()
	InjectLegadoStubs(ctx)
	v, err := ctx.Eval("test.js", qjs.Code("typeof cookie"))
	if err != nil {
		t.Fatalf("typeof cookie error: %v", err)
	}
	if v.String() != "object" {
		t.Errorf("Expected cookie to be object, got %q", v.String())
	}
	v.Free()
}

func TestStub_SourceDefined(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	rt, err := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Put(rt)
	ctx := rt.Context()
	InjectLegadoStubs(ctx)
	v, err := ctx.Eval("test.js", qjs.Code("typeof source.getKey"))
	if err != nil {
		t.Fatalf("typeof source.getKey error: %v", err)
	}
	if v.String() != "function" {
		t.Errorf("Expected source.getKey to be function, got %q", v.String())
	}
	v.Free()
}

func TestStub_CreateRegExp(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	rt, err := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Put(rt)
	ctx := rt.Context()
	InjectLegadoStubs(ctx)
	v, err := ctx.Eval("test.js", qjs.Code("createRegExp('test[1]').toString()"))
	if err != nil {
		t.Fatalf("createRegExp error: %v", err)
	}
	if v.String() != "/test[1]/i" {
		t.Errorf("Expected /test\\[1\\]/i, got %q", v.String())
	}
	v.Free()
}

func TestStub_JavaGetElements(t *testing.T) {
	pool := qjs.NewPool(1, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	rt, err := pool.Get()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Put(rt)
	ctx := rt.Context()
	InjectLegadoStubs(ctx)
	// Set HTML content via java.setContent
	_, _ = ctx.Eval("set.js", qjs.Code(`java.setContent("<div><a href='/1'>A</a><a href='/2'>B</a></div>")`))
	v, err := ctx.Eval("test.js", qjs.Code(`java.getElements("a").length`))
	if err != nil {
		t.Fatalf("java.getElements error: %v", err)
	}
	if v.Int32() != 2 {
		t.Errorf("Expected 2 elements, got %d", v.Int32())
	}
	v.Free()
}
