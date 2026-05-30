package analyzeurl

import (
	"testing"
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
	// Simulates qidian searchUrl: @js:url=baseUrl+"/so/{{key}}.html,{'method':'GET',...}";result=url;
	ruleUrl := `@js:url=baseUrl+"/so/{{key}}.html,{'method':'GET','headers':{'User-Agent':'TestBot','Referer':'https://www.qidian.com/'}}";java.put('url',url);result=url;`
	a := New(ruleUrl, "斗破", 1, "https://www.qidian.com", "")
	// initUrl should strip the @js: and store jsCode
	if a.jsCode == "" {
		t.Fatal("Expected jsCode to be set for @js: prefix")
	}
	if a.FinalUrl != "" {
		t.Logf("FinalUrl before JS exec: %s", a.FinalUrl)
	}
}
