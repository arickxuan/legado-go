package analyzer

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fastschema/qjs"
)

// legadoStubJS defines JavaScript stubs for Legado's Java bridge APIs.
// These allow book sources that use <js> blocks to run without crashing.
const legadoStubJS = `
var java = {
  _store: {},
  _content: "",
  put: function(k, v) { this._store[k] = String(v); return String(v); },
  get: function(k) { return this._store[k] || ""; },
  log: function() { /* silent */ },
  toast: function() { /* silent */ },
  longToast: function() { /* silent */ },
  encodeURI: function(s) { return encodeURI(s); },
  base64Decode: function(s) { return __legado_base64_decode(String(s)); },
  base64Encode: function(s) { return __legado_base64_encode(String(s)); },
  md5Encode: function(s) { return __legado_md5(String(s)); },
  md5Encode16: function(s) { return __legado_md5(String(s)).substring(8, 24); },
  hexDecodeToString: function(hex) {
    var s = "";
    for (var i = 0; i < hex.length; i += 2) {
      s += String.fromCharCode(parseInt(hex.substr(i, 2), 16));
    }
    return s;
  },
  s2t: function(s) { return s; },
  t2s: function(s) { return s; },
  ajax: function(url) { return __legado_ajax(String(url)); },
  connect: function(url) { return __legado_make_response(__legado_request("GET", String(url), "", "{}")); },
  get: function(url, headers) { return __legado_make_response(__legado_request("GET", String(url), "", JSON.stringify(headers || {}))); },
  post: function(url, body, headers) { return __legado_make_response(__legado_request("POST", String(url), String(body || ""), JSON.stringify(headers || {}))); },
  head: function(url, headers) { return __legado_make_response(__legado_request("HEAD", String(url), "", JSON.stringify(headers || {}))); },
  setContent: function(html) { this._content = html; },
  getContent: function() { return this._content; },
  getElements: function(path) { return []; },
  getString: function(path) { return ""; },
  getStringList: function(path) { return []; },
  webView: function(html, url, js) { return html || this.ajax(url || ""); },
  startBrowserAwait: function(url, msg) { return ""; },
  putLoginInfo: function(msg) {},
  getVerificationCode: function(url) { return ""; },
  getCookie: function(url, name) { return ""; },
  timeFormat: function(ts) { return new Date(Number(ts)).toISOString(); },
  androidId: function() { return ""; }
};
var cookie = {
  removeCookie: function(key) {},
  getCookie: function(key) { return ""; },
  setCookie: function(key, value) {}
};
var cache = {
  put: function(key, value, seconds) { __legado_cache_put(String(key), String(value)); return String(value); },
  get: function(key) { return __legado_cache_get(String(key)); },
  delete: function(key) { __legado_cache_delete(String(key)); }
};
var source = {
  _var: "",
  _comment: "",
  bookSourceComment: "",
  bookSourceName: "",
  searchUrl: "",
  bookList: "",
  getKey: function() { return ""; },
  getVariable: function() { return this._var; },
  setVariable: function(v) { this._var = String(v); },
  put: function(k, v) { java._store[String(k)] = String(v); return String(v); },
  get: function(k) { return java._store[String(k)] || ""; },
  putLoginInfo: function(msg) {},
  getLoginInfoMap: function() { return {}; }
};
function __legado_make_response(json) {
  var r = {};
  try { r = JSON.parse(String(json || "{}")); } catch(e) { r = {body:String(json||""), headers:{}, code:0, url:""}; }
  r.headers = r.headers || {};
  return {
    body: function(){ return r.body || ""; },
    code: function(){ return Number(r.code || 0); },
    statusCode: function(){ return Number(r.code || 0); },
    message: function(){ return r.status || ""; },
    url: function(){ return r.url || ""; },
    header: function(k){ return r.headers[String(k).toLowerCase()] || r.headers[String(k)] || ""; },
    headers: function(){ return { get: function(k){ return r.headers[String(k).toLowerCase()] || r.headers[String(k)] || ""; } }; },
    raw: function(){ return r; },
    toString: function(){ return r.body || ""; }
  };
}
function checkData() { return {}; }
function toDataUrl(key, page) { return ""; }
function getl(arr, page) {
  var size = 20;
  var start = page * size;
  return arr.slice(start, start + size);
}
function createRegExp(key) {
  try { return new RegExp(key, "i"); }
  catch(e) { return new RegExp(key.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), "i"); }
}
function getSortValue(item, st) { return item[st] || 0; }
`

var jsCache sync.Map

type jsHTTPResponse struct {
	Body    string            `json:"body"`
	Code    int               `json:"code"`
	Status  string            `json:"status"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// InjectLegadoStubs injects Legado API stubs into a JS context.
func InjectLegadoStubs(ctx *qjs.Context) {
	// Register Go-backed ajax function for real HTTP requests
	ctx.SetFunc("__legado_ajax", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewString(""), nil
		}
		body, err := doLegadoAjax(args[0].String())
		if err != nil {
			return ctx.NewString(""), nil
		}
		return ctx.NewString(body), nil
	})

	ctx.SetFunc("__legado_request", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		method, urlStr, body, headersJSON := "GET", "", "", "{}"
		if len(args) > 0 {
			method = args[0].String()
		}
		if len(args) > 1 {
			urlStr = args[1].String()
		}
		if len(args) > 2 {
			body = args[2].String()
		}
		if len(args) > 3 {
			headersJSON = args[3].String()
		}
		resp := doLegadoRequest(method, urlStr, body, parseHeadersJSON(headersJSON), false)
		b, _ := json.Marshal(resp)
		return ctx.NewString(string(b)), nil
	})

	ctx.SetFunc("__legado_base64_decode", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewString(""), nil
		}
		b, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(args[0].String())
		}
		if err != nil {
			return ctx.NewString(""), nil
		}
		return ctx.NewString(string(b)), nil
	})

	ctx.SetFunc("__legado_base64_encode", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewString(""), nil
		}
		return ctx.NewString(base64.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	})

	ctx.SetFunc("__legado_md5", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewString(""), nil
		}
		sum := md5.Sum([]byte(args[0].String()))
		return ctx.NewString(hex.EncodeToString(sum[:])), nil
	})

	ctx.SetFunc("__legado_cache_put", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) >= 2 {
			jsCache.Store(args[0].String(), args[1].String())
		}
		return ctx.NewUndefined(), nil
	})
	ctx.SetFunc("__legado_cache_get", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewNull(), nil
		}
		if v, ok := jsCache.Load(args[0].String()); ok {
			return ctx.NewString(v.(string)), nil
		}
		return ctx.NewNull(), nil
	})
	ctx.SetFunc("__legado_cache_delete", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) > 0 {
			jsCache.Delete(args[0].String())
		}
		return ctx.NewUndefined(), nil
	})

	ctx.SetFunc("__legado_log", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		parts := make([]interface{}, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		fmt.Println(parts...)
		return ctx.NewUndefined(), nil
	})

	// getElements: parse HTML content with CSS selectors, return JS array
	ctx.SetFunc("__legado_getElements", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewArray().Value, nil
		}
		rule := args[0].String()
		// Get stored content from java._content
		javaObj := ctx.Global().GetPropertyStr("java")
		var content string
		if javaObj != nil {
			cv := javaObj.GetPropertyStr("_content")
			if cv != nil && !cv.IsUndefined() && !cv.IsNull() {
				content = cv.String()
				cv.Free()
			}
			javaObj.Free()
		}
		if content == "" {
			// Fall back to global 'result' variable (response body)
			rv := ctx.Global().GetPropertyStr("result")
			if rv != nil && !rv.IsUndefined() && !rv.IsNull() {
				content = rv.String()
				rv.Free()
			}
		}
		if content == "" {
			return ctx.NewArray().Value, nil
		}
		// Split by || for multiple selectors
		selectors := strings.Split(rule, "||")
		var allResults []string
		for _, sel := range selectors {
			sel = strings.TrimSpace(sel)
			if sel == "" {
				continue
			}
			results := cssGetElementList(sel, content, "")
			allResults = append(allResults, results...)
		}
		// Build JS array
		arr := ctx.NewArray()
		for i, r := range allResults {
			arr.SetPropertyIndex(int64(i), ctx.NewString(r))
		}
		return arr.Value, nil
	})

	// getString: parse HTML content with CSS rule, return first text match
	ctx.SetFunc("__legado_getString", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewString(""), nil
		}
		rule := args[0].String()
		javaObj := ctx.Global().GetPropertyStr("java")
		var content string
		if javaObj != nil {
			cv := javaObj.GetPropertyStr("_content")
			if cv != nil && !cv.IsUndefined() && !cv.IsNull() {
				content = cv.String()
				cv.Free()
			}
			javaObj.Free()
		}
		if content == "" {
			rv := ctx.Global().GetPropertyStr("result")
			if rv != nil && !rv.IsUndefined() && !rv.IsNull() {
				content = rv.String()
				rv.Free()
			}
		}
		if content == "" {
			return ctx.NewString(""), nil
		}
		result := cssGetString(rule, content, "")
		return ctx.NewString(result), nil
	})

	_, _ = ctx.Eval("legado_stub.js", qjs.Code(legadoStubJS))

	// Override java methods with Go implementations
	_, _ = ctx.Eval("legado_override.js", qjs.Code(`
		java.ajax = __legado_ajax;
		java.log = __legado_log;
		java.getElements = __legado_getElements;
		java.getString = __legado_getString;
	`))
}

// InjectLegadoStubsToPool prepares a JS runtime with Legado stubs.
// Call this in the Pool's init function.
func InjectLegadoStubsToPool(r *qjs.Runtime) error {
	return nil // stubs are injected per-context via InjectLegadoStubs
}

func doLegadoAjax(urlStr string) (string, error) {
	method, target, body, headers := parseJSURL(urlStr)
	resp := doLegadoRequest(method, target, body, headers, true)
	return resp.Body, nil
}

func doLegadoRequest(method string, urlStr string, body string, headers map[string]string, followRedirect bool) jsHTTPResponse {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "GET"
	}
	targetMethod, targetURL, targetBody, targetHeaders := parseJSURL(urlStr)
	if targetMethod != "" {
		method = targetMethod
	}
	if targetBody != "" && body == "" {
		body = targetBody
	}
	for k, v := range targetHeaders {
		if _, ok := headers[k]; !ok {
			headers[k] = v
		}
	}
	result := jsHTTPResponse{Headers: map[string]string{}, URL: targetURL}
	if strings.TrimSpace(targetURL) == "" {
		result.Status = "empty url"
		return result
	}
	client := &http.Client{Timeout: 15 * time.Second}
	if !followRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	var reader io.Reader
	if method == "POST" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, targetURL, reader)
	if err != nil {
		result.Status = err.Error()
		return result
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if method == "POST" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := client.Do(req)
	if err != nil {
		result.Status = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.Code = resp.StatusCode
	result.Status = resp.Status
	if resp.Request != nil && resp.Request.URL != nil {
		result.URL = resp.Request.URL.String()
	}
	for k, values := range resp.Header {
		if len(values) > 0 {
			result.Headers[strings.ToLower(k)] = values[0]
			result.Headers[k] = values[0]
		}
	}
	b, err := io.ReadAll(resp.Body)
	if err == nil {
		result.Body = string(b)
	}
	return result
}

func parseJSURL(urlStr string) (method string, target string, body string, headers map[string]string) {
	headers = map[string]string{}
	method = "GET"
	target = strings.TrimSpace(urlStr)
	if idx := findJSOptionSeparator(target); idx >= 0 {
		option := strings.TrimSpace(target[idx+1:])
		target = strings.TrimSpace(target[:idx])
		var opt map[string]any
		if err := json.Unmarshal([]byte(normalizeJSJSON(option)), &opt); err == nil {
			if v, ok := opt["method"].(string); ok && v != "" {
				method = strings.ToUpper(v)
			}
			if v, ok := opt["body"]; ok {
				body = stringifyJSValue(v)
			}
			if v, ok := opt["headers"]; ok {
				if m, ok := v.(map[string]any); ok {
					for hk, hv := range m {
						headers[hk] = stringifyJSValue(hv)
					}
				}
			}
		}
	}
	return method, target, body, headers
}

func findJSOptionSeparator(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			rest := strings.TrimSpace(s[i+1:])
			if strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "'") || strings.HasPrefix(rest, "\"") {
				return i
			}
		}
	}
	return -1
}

func normalizeJSJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "'") {
		var b strings.Builder
		inDouble := false
		for i := 0; i < len(s); i++ {
			ch := s[i]
			if ch == '"' {
				inDouble = !inDouble
				b.WriteByte(ch)
			} else if ch == '\'' && !inDouble {
				b.WriteByte('"')
			} else {
				b.WriteByte(ch)
			}
		}
		s = b.String()
	}
	return s
}

func stringifyJSValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case nil:
		return ""
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		return string(b)
	}
}

func parseHeadersJSON(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		out[k] = stringifyJSValue(v)
	}
	return out
}
