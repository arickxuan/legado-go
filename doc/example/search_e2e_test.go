package example

import (
	"strings"
	"testing"

	"gopro/analyzer"
	"gopro/analyzeurl"
	"gopro/model"

	"github.com/fastschema/qjs"
)

// ============================================================
// Search URL Template Resolution Tests
// ============================================================

func newPool() *qjs.Pool {
	return qjs.NewPool(2, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
}

func TestSearchUrl_PostWithCharset(t *testing.T) {
	// 第一版主i12 pattern: /s.php with GBK POST body
	pool := newPool()
	searchUrl := `/s.php,{"method":"POST","body":"type=articlename&s={{key}}","charset":"gbk"}`
	a := analyzeurl.New(searchUrl, "斗破", 1, "http://i.12bz.net", "", pool)
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if a.Charset != "gbk" {
		t.Errorf("Expected gbk charset, got %s", a.Charset)
	}
	if !strings.Contains(a.Body, "斗破") {
		t.Errorf("Body should contain raw key, got %q", a.Body)
	}
}

func TestSearchUrl_AngleBracketPage(t *testing.T) {
	// 涩涩小说 pattern: /book/search/{{key}}<,/{{page}}>
	pool := newPool()
	searchUrl := "/book/search/{{key}}<,/{{page}}>"
	// page=1: no page suffix
	a := analyzeurl.New(searchUrl, "斗破", 1, "https://sesebooks.com", "", pool)
	if strings.Contains(a.FinalUrl, "<") || strings.Contains(a.FinalUrl, ">") {
		t.Errorf("Angle brackets should be resolved, got %q", a.FinalUrl)
	}
	if strings.Contains(a.FinalUrl, "page") {
		t.Errorf("page=1 should have no page suffix, got %q", a.FinalUrl)
	}
	// page=2: should have /2
	a2 := analyzeurl.New(searchUrl, "斗破", 2, "https://sesebooks.com", "", pool)
	if !strings.Contains(a2.FinalUrl, "/2") {
		t.Errorf("page=2 should contain /2, got %q", a2.FinalUrl)
	}
}

func TestSearchUrl_CookieRemoveCookie(t *testing.T) {
	// 要撸小说 pattern: {{cookie.removeCookie("...")}}\nURL,{...}
	pool := newPool()
	searchUrl := `{{cookie.removeCookie("https://yaoluku.com")}}
https://yaoluku.com/search/,{"body":"searchkey={{key}}&searchtype=all&Submit=","method":"POST"}`
	a := analyzeurl.New(searchUrl, "干妈", 1, "https://yaoluxs.com", "", pool)
	if strings.Contains(a.FinalUrl, "cookie") {
		t.Errorf("cookie.removeCookie should be resolved, got %q", a.FinalUrl)
	}
	if strings.Contains(a.FinalUrl, "{{") {
		t.Errorf("All templates should be resolved, got %q", a.FinalUrl)
	}
	if a.Method != "POST" {
		t.Errorf("Expected POST method, got %s", a.Method)
	}
	if !strings.Contains(a.Body, "干妈") {
		t.Errorf("Body should contain raw key, got %q", a.Body)
	}
}

func TestSearchUrl_SourceCommentEval(t *testing.T) {
	// 奇怪小说 pattern: {{cookie.removeCookie(source.getKey());eval(String(source.bookSourceComment))}}
	pool := newPool()
	searchUrl := "{{cookie.removeCookie(source.getKey());eval(String(source.bookSourceComment))}}"
	comment := "result = 'https://www.aakkrr.com/book/' + encodeURIComponent(key) + '<,/1>'"
	a := analyzeurl.New(searchUrl, "斗破", 1, "https://www.aahhss.com", "", pool)
	a.SetComment(comment)
	if strings.Contains(a.FinalUrl, "{{") {
		t.Errorf("All templates should be resolved after SetComment, got %q", a.FinalUrl)
	}
	if !strings.Contains(a.FinalUrl, "aakkrr.com") {
		t.Errorf("Should resolve to comment URL, got %q", a.FinalUrl)
	}
}

func TestSearchUrl_BareJsPrefix(t *testing.T) {
	// 八叉书库 pattern: @js:...
	pool := newPool()
	searchUrl := `@js:
let new_url = java.get(baseUrl + '/e/search/index.php?keyboard=' + key);
result = new_url;
`
	a := analyzeurl.New(searchUrl, "斗破", 1, "https://bcshuku.com", "", pool)
	// With @js: prefix, FinalUrl should be empty until GetStrResponse executes JS
	if a.FinalUrl != "" && a.FinalUrl != "https://bcshuku.com" {
		t.Logf("Note: FinalUrl=%q (JS executed in GetStrResponse)", a.FinalUrl)
	}
}

func TestSearchUrl_JsPrefix(t *testing.T) {
	// 第一版主999 pattern: <js>...</js> followed by URL
	pool := newPool()
	searchUrl := `<js>
try { data = JSON.parse(source.getVariable()); } catch (err) { data = {}; }
let url = data.url || baseUrl;
url + "s.php," + JSON.stringify({"charset":"GBK","method":"POST","body":"objectType=2&type=articlename&s=" + key});
</js>`
	a := analyzeurl.New(searchUrl, "斗破", 1, "https://www.banzhu44444.com/", "", pool)
	t.Logf("JS prefix result: FinalUrl=%q Method=%s", a.FinalUrl, a.Method)
}

func TestSearchUrl_QueryParamKey(t *testing.T) {
	// 爱丽丝书屋 pattern: search.html?q={{key}}&p={{page}}
	pool := newPool()
	searchUrl := "https://www.alicesw.com/search.html?q={{key}}&p={{page}}"
	a := analyzeurl.New(searchUrl, "斗破", 2, "https://www.alicesw.com", "", pool)
	if !strings.Contains(a.FinalUrl, "q=") {
		t.Errorf("Should contain query param, got %q", a.FinalUrl)
	}
	if !strings.Contains(a.FinalUrl, "p=2") {
		t.Errorf("Should contain page=2, got %q", a.FinalUrl)
	}
}

func TestSearchUrl_RelativePath(t *testing.T) {
	// 🔞奇怪小说 pattern: /e/search/,{...}
	pool := newPool()
	searchUrl := `/e/search/,{"body":"keyboard={{key}}","method":"POST"}`
	a := analyzeurl.New(searchUrl, "斗破", 1, "https://www.aahhss.com/", "", pool)
	if !strings.HasPrefix(a.FinalUrl, "https://www.aahhss.com") {
		t.Errorf("Should resolve relative to source URL, got %q", a.FinalUrl)
	}
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
}

// ============================================================
// CSS/HTML Rule Tests (search result parsing)
// ============================================================

const aliceSearchHTML = `<html><body>
<div class="list-group">
  <div class="list-group-item">
    <h5><a href="/book/1/">1.斗破苍穹[已完结]</a></h5>
    <p class="mb-1"><a href="/author/1/">天蚕土豆</a></p>
    <span class="s4">玄幻</span>
  </div>
  <div class="list-group-item">
    <h5><a href="/book/2/">2.武动乾坤[连载中]</a></h5>
    <p class="mb-1"><a href="/author/2/">天蚕土豆</a></p>
    <span class="s4">玄幻</span>
  </div>
</div>
</body></html>`

func TestCSS_AliceswBookList(t *testing.T) {
	pool := newPool()
	ar := analyzer.NewAnalyzeRule(aliceSearchHTML, "https://www.alicesw.com/search.html", pool)
	items := ar.GetElements("class.list-group-item")
	if len(items) != 2 {
		t.Fatalf("Expected 2 list-group-items, got %d", len(items))
	}
	// Check first item
	item := analyzer.NewAnalyzeRule(items[0], "https://www.alicesw.com", pool)
	name := item.GetString("h5@text##\\d+\\.|\\[已完结\\]|\\[连载中\\]")
	if !strings.Contains(name, "斗破苍穹") {
		t.Errorf("Expected name to contain 斗破苍穹, got %q", name)
	}
	author := item.GetString("class.mb-1@a@text")
	if author != "天蚕土豆" {
		t.Errorf("Expected author 天蚕土豆, got %q", author)
	}
}

const diyibanzhuHTML = `<html><body>
<div class="mod block book-all-list">
  <div class="bd">
    <div class="right">
      <a class="name" href="/book/100/">斗破苍穹</a>
      <span class="author">天蚕土豆</span>
    </div>
    <div class="right">
      <a class="name" href="/book/200/">武动乾坤</a>
      <span class="author">天蚕土豆</span>
    </div>
  </div>
</div>
</body></html>`

func TestCSS_DiyibanzhuBookList(t *testing.T) {
	pool := newPool()
	ar := analyzer.NewAnalyzeRule(diyibanzhuHTML, "https://m.diyibanzhu2.online/", pool)
	// 第一版主网 ruleSearch.bookList: class.mod block book-all-list@class.bd@tag.li@class.right
	items := ar.GetElements("class.mod block book-all-list@class.bd@class.right")
	if len(items) != 2 {
		t.Fatalf("Expected 2 right items, got %d", len(items))
	}
	item := analyzer.NewAnalyzeRule(items[0], "https://m.diyibanzhu2.online/", pool)
	name := item.GetString("class.name@text")
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹, got %q", name)
	}
	author := item.GetString("class.author@text")
	if author != "天蚕土豆" {
		t.Errorf("Expected 天蚕土豆, got %q", author)
	}
}

const coverHTML = `<html><body>
<div class="cover">
  <p><a href="/s.php">First</a><a href="/book/1/">斗破苍穹</a><a href="/author/">天蚕土豆</a></p>
  <p><a href="/s.php">First</a><a href="/book/2/">武动乾坤</a><a href="/author/">土豆</a></p>
</div>
</body></html>`

func TestCSS_CoverP(t *testing.T) {
	pool := newPool()
	ar := analyzer.NewAnalyzeRule(coverHTML, "http://i.12bz.net", pool)
	// 第一版主net bookList: .cover p
	items := ar.GetElements(".cover@p")
	if len(items) != 2 {
		t.Fatalf("Expected 2 p items, got %d", len(items))
	}
	item := analyzer.NewAnalyzeRule(items[0], "http://i.12bz.net", pool)
	// name: a.1@text (second <a> tag's text)
	name := item.GetString("tag.a.1@text")
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹 from a.1@text, got %q", name)
	}
}

const lineHTML = `<html><body>
<div class="line">
  <a href="/1">Index</a>
  <a href="/book/1/">斗破苍穹</a>
  <a href="/author/">天蚕土豆</a>
</div>
<div class="line">
  <a href="/2">Index</a>
  <a href="/book/2/">武动乾坤</a>
  <a href="/author/">土豆</a>
</div>
</body></html>`

func TestCSS_LineItems(t *testing.T) {
	pool := newPool()
	ar := analyzer.NewAnalyzeRule(lineHTML, "http://i.12bz.net", pool)
	items := ar.GetElements("class.line")
	if len(items) != 2 {
		t.Fatalf("Expected 2 line items, got %d", len(items))
	}
	item := analyzer.NewAnalyzeRule(items[0], "http://i.12bz.net", pool)
	name := item.GetString("class.line@tag.a.1@text")
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹, got %q", name)
	}
}

// ============================================================
// JSON Rule Tests (search result parsing)
// ============================================================

const searchJSON = `[{"name":"斗破苍穹","author":"天蚕土豆","cover":"/c/1.jpg","intro":"萧炎的传奇","kind":"玄幻","lastChapter":"第一千两百章","wordCount":"322万"},{"name":"武动乾坤","author":"天蚕土豆","cover":"/c/2.jpg","intro":"林动的故事","kind":"玄幻","lastChapter":"第八百章","wordCount":"280万"}]`

func TestJSON_SearchBookList(t *testing.T) {
	pool := newPool()
	ar := analyzer.NewAnalyzeRule(searchJSON, "https://api.example.com/search", pool)
	items := ar.GetElements("$[*]")
	if len(items) != 2 {
		t.Fatalf("Expected 2 JSON items, got %d", len(items))
	}
	item := analyzer.NewAnalyzeRule(items[0], "https://api.example.com", pool)
	name := item.GetString("$.name")
	if name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹, got %q", name)
	}
	author := item.GetString("$.author")
	if author != "天蚕土豆" {
		t.Errorf("Expected 天蚕土豆, got %q", author)
	}
	kind := item.GetString("$.kind")
	if kind != "玄幻" {
		t.Errorf("Expected 玄幻, got %q", kind)
	}
}

// ============================================================
// Full Search Flow Test
// ============================================================

func TestSearchFromSource_CSS(t *testing.T) {
	// This tests the analyzer rule parsing chain with a mock HTML response
	pool := newPool()
	body := aliceSearchHTML
	ar := analyzer.NewAnalyzeRule(body, "https://www.alicesw.com/search.html", pool)
	bookListRule := "class.list-group-item"
	items := ar.GetElements(bookListRule)
	if len(items) == 0 {
		t.Fatal("Expected book items from CSS rule")
	}
	for _, item := range items {
		itemAr := analyzer.NewAnalyzeRule(item, "https://www.alicesw.com", pool)
		name := itemAr.GetString("h5@text##\\d+\\.|\\[已完结\\]|\\[连载中\\]")
		if name == "" {
			t.Error("Book name should not be empty")
		}
		t.Logf("Found book: %s", name)
	}
}

func TestSearchFromSource_JSON(t *testing.T) {
	pool := newPool()
	ar := analyzer.NewAnalyzeRule(searchJSON, "https://api.example.com/search", pool)
	items := ar.GetElements("$[*]")
	if len(items) == 0 {
		t.Fatal("Expected book items from JSON rule")
	}
	for _, item := range items {
		itemAr := analyzer.NewAnalyzeRule(item, "https://api.example.com", pool)
		name := itemAr.GetString("{{@h4@text||h3@text||\"\"}}{{$.name||\"\"}}")
		author := itemAr.GetString("{{@span@text||\"\"}}{{$.author||\"\"}}")
		t.Logf("Found book: %s by %s", name, author)
		if name == "" {
			t.Error("Book name should not be empty")
		}
	}
}

// ============================================================
// Edge Cases
// ============================================================

func TestSearchUrl_EmptyComment(t *testing.T) {
	pool := newPool()
	searchUrl := "{{cookie.removeCookie(source.getKey());eval(String(source.bookSourceComment))}}"
	a := analyzeurl.New(searchUrl, "test", 1, "https://example.com", "", pool)
	// SetComment with empty string should not crash
	a.SetComment("")
	// FinalUrl may be empty or just the source URL — that's OK
	t.Logf("Empty comment FinalUrl=%q", a.FinalUrl)
}

func TestSearchUrl_DoublePagePattern(t *testing.T) {
	pool := newPool()
	// 第一版主网 pattern: search_top_{{key}}_691_{{page}}.html
	searchUrl := "search_top_{{key}}_691_{{page}}.html"
	a := analyzeurl.New(searchUrl, "斗破", 3, "https://m.diyibanzhu2.online/", "", pool)
	if !strings.Contains(a.FinalUrl, "3.html") {
		t.Errorf("Should contain page 3, got %q", a.FinalUrl)
	}
	if !strings.Contains(a.FinalUrl, "691") {
		t.Errorf("Should contain 691, got %q", a.FinalUrl)
	}
}

func TestSearchUrl_GBKPostBody(t *testing.T) {
	pool := newPool()
	// 龙腾小说城 pattern: /s.php with GBK POST
	searchUrl := `/s.php,{"body":"type=articlename&s={{key}}","charset":"GBK","method":"POST"}`
	a := analyzeurl.New(searchUrl, "斗破", 1, "https://m.longtengxiaoshuo.org", "", pool)
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if strings.ToLower(a.Charset) != "gbk" {
		t.Errorf("Expected GBK charset, got %s", a.Charset)
	}
	// Body should contain raw key (not URL-encoded)
	if !strings.Contains(a.Body, "斗破") {
		t.Errorf("Body should contain raw key 斗破, got %q", a.Body)
	}
}

func TestSearchUrl_SingleQuoteOption(t *testing.T) {
	pool := newPool()
	searchUrl := `/search/,{'method':'POST','body':'searchkey={{key}}'}`
	a := analyzeurl.New(searchUrl, "test", 1, "https://example.com", "", pool)
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if a.Body != "searchkey=test" {
		t.Errorf("Expected body 'searchkey=test', got %q", a.Body)
	}
}

func TestSearchModel_SearchBook(t *testing.T) {
	// Verify SearchBook struct fields work correctly
	book := model.SearchBook{
		Name:       "斗破苍穹",
		Author:     "天蚕土豆",
		CoverUrl:   "/c/1.jpg",
		Intro:      "萧炎的传奇",
		Kind:       "玄幻",
		BookUrl:    "/book/1/",
		Origin:     "https://example.com",
		OriginName: "测试源",
	}
	if book.Name != "斗破苍穹" {
		t.Errorf("Expected 斗破苍穹, got %s", book.Name)
	}
	if book.Author != "天蚕土豆" {
		t.Errorf("Expected 天蚕土豆, got %s", book.Author)
	}
}
