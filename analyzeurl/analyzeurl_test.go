package analyzeurl

import (
	"strings"
	"testing"

	"github.com/fastschema/qjs"
)

func TestParseUrlOptionSingleQuote(t *testing.T) {
	a := &AnalyzeUrl{Method: "GET", HeaderMap: make(map[string]string)}
	a.parseUrlOption(`{'method':'POST','body':'s=test'}`)
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if a.Body != "s=test" {
		t.Errorf("Expected 's=test', got '%s'", a.Body)
	}
}

func TestParseUrlOptionDoubleQuote(t *testing.T) {
	a := &AnalyzeUrl{Method: "GET", HeaderMap: make(map[string]string)}
	a.parseUrlOption(`{"method":"POST","body":"s=hello"}`)
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if a.Body != "s=hello" {
		t.Errorf("Expected 's=hello', got '%s'", a.Body)
	}
}

func TestParseUrlOptionWithHeaders(t *testing.T) {
	a := &AnalyzeUrl{Method: "GET", HeaderMap: make(map[string]string)}
	a.parseUrlOption(`{'method':'GET','headers':{'User-Agent':'TestBot'}}`)
	if a.Method != "GET" {
		t.Errorf("Expected GET, got %s", a.Method)
	}
	if a.HeaderMap["User-Agent"] != "TestBot" {
		t.Errorf("Expected TestBot header, got '%s'", a.HeaderMap["User-Agent"])
	}
}

func TestInitUrlWithPostBody(t *testing.T) {
	a := New(`/search.html,{'method':'POST','body':'s={{key}}'}`, "test", 1, "https://www.50zw.so", "")
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if a.Body != "s=test" {
		t.Errorf("Expected 's=test', got '%s'", a.Body)
	}
	if a.FinalUrl != "https://www.50zw.so/search.html" {
		t.Errorf("Expected https://www.50zw.so/search.html, got %s", a.FinalUrl)
	}
}

func TestSingleToDoubleQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`{'a':'b'}`, `{"a":"b"}`},
		{`{"a":"b"}`, `{"a":"b"}`},
		{`{'method':'POST','body':'s=test'}`, `{"method":"POST","body":"s=test"}`},
	}
	for _, tt := range tests {
		got := singleToDoubleQuote(tt.input)
		if got != tt.want {
			t.Errorf("singleToDoubleQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInitUrlBareJsPrefix(t *testing.T) {
	ruleUrl := `@js:url=baseUrl+"/so/{{key}}.html,{'method':'GET','headers':{'User-Agent':'TestBot','Referer':'https://www.qidian.com/'}}";java.put('url',url);result=url;`
	a := New(ruleUrl, "斗破", 1, "https://www.qidian.com", "")
	if a.jsCode == "" {
		t.Fatal("Expected jsCode to be set for @js: prefix")
	}
	if a.FinalUrl != "" {
		t.Logf("FinalUrl before JS exec: %s", a.FinalUrl)
	}
}

// --- New tests for fixes ---

func TestInlineExpr_CookieRemoveCookie(t *testing.T) {
	// {{cookie.removeCookie(...)}} should evaluate to empty and be stripped
	pool := qjs.NewPool(2, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	searchUrl := "{{cookie.removeCookie(\"https://example.com\")}}\nhttps://example.com/search/,{\"body\":\"s={{key}}\",\"method\":\"POST\"}"
	a := New(searchUrl, "test", 1, "https://example.com", "", pool)
	a.SetComment("")
	if strings.Contains(a.FinalUrl, "cookie") {
		t.Errorf("Expected cookie.removeCookie to be resolved, got FinalUrl=%q", a.FinalUrl)
	}
	if a.Method != "POST" {
		t.Errorf("Expected POST method, got %s", a.Method)
	}
	if a.Body != "s=test" {
		t.Errorf("Expected body 's=test', got %q", a.Body)
	}
	t.Logf("FinalUrl=%q Method=%s Body=%q", a.FinalUrl, a.Method, a.Body)
}

func TestInlineExpr_SourceComment(t *testing.T) {
	// {{eval(String(source.bookSourceComment))}} should evaluate the comment JS
	pool := qjs.NewPool(2, qjs.Option{}, func(r *qjs.Runtime) error { return nil })
	searchUrl := "{{cookie.removeCookie(source.getKey());eval(String(source.bookSourceComment))}}"
	comment := "result = 'https://example.com/search/'"
	a := New(searchUrl, "test", 1, "https://example.com", "", pool)
	a.SetComment(comment)
	if a.FinalUrl != "https://example.com/search/" {
		t.Errorf("Expected resolved URL, got %q", a.FinalUrl)
	}
}

func TestAngleBracketPages(t *testing.T) {
	// <,/${page}> syntax: page=1 should remove the block
	a := &AnalyzeUrl{page: 1}
	result := a.resolveAngleBracketPages("/book/test<,/${page}>")
	if strings.Contains(result, "<") || strings.Contains(result, ">") {
		t.Errorf("Expected angle brackets resolved for page 1, got %q", result)
	}
	// page=2 should keep the value
	a.page = 2
	result = a.resolveAngleBracketPages("/book/test<,/${page}>")
	if !strings.Contains(result, "/2") {
		t.Errorf("Expected /2 for page 2, got %q", result)
	}
}

func TestFindTemplateExpr(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"{{expr1}}rest", "expr1"},
		{"prefix{{expr2}}suffix", "expr2"},
		{"no template here", ""},
		{"{{a;b}}", "a;b"},
	}
	for _, tt := range tests {
		_, _, got := findTemplateExpr(tt.input)
		if got != tt.want {
			t.Errorf("findTemplateExpr(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGbkCharset_OptionParsed(t *testing.T) {
	a := New(`/s.php,{"charset":"gbk","method":"POST","body":"s={{key}}"}`, "test", 1, "https://example.com", "")
	if a.Charset != "gbk" {
		t.Errorf("Expected charset gbk, got %q", a.Charset)
	}
	if a.Method != "POST" {
		t.Errorf("Expected POST, got %s", a.Method)
	}
	if a.Body != "s=test" {
		t.Errorf("Expected body 's=test', got %q", a.Body)
	}
}

func TestGbkCharset_RawKeyInBody(t *testing.T) {
	// When charset is GBK, body should have raw key (not URL-encoded)
	a := New(`/s.php,{"charset":"gbk","method":"POST","body":"s={{key}}"}`, "斗破", 1, "https://example.com", "")
	if strings.Contains(a.Body, "%") {
		t.Errorf("Expected raw key in body for GBK charset, got %q", a.Body)
	}
	if a.Body != "s=斗破" {
		t.Errorf("Expected 's=斗破', got %q", a.Body)
	}
}
