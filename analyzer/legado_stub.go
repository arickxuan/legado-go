package analyzer

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fastschema/qjs"
)

// legadoStubJS defines JavaScript stubs for Legado's Java bridge APIs.
// These allow book sources that use <js> blocks to run without crashing.
const legadoStubJS = `
var java = {
  _store: {},
  _content: "",
  put: function(k, v) { this._store[k] = String(v); },
  get: function(k) { return this._store[k] || ""; },
  log: function() { /* silent */ },
  toast: function() { /* silent */ },
  longToast: function() { /* silent */ },
  encodeURI: function(s) { return encodeURI(s); },
  hexDecodeToString: function(hex) {
    var s = "";
    for (var i = 0; i < hex.length; i += 2) {
      s += String.fromCharCode(parseInt(hex.substr(i, 2), 16));
    }
    return s;
  },
  s2t: function(s) { return s; },
  t2s: function(s) { return s; },
  ajax: function(url) {
    return "";
  },
  setContent: function(html) { this._content = html; },
  getContent: function() { return this._content; },
  getElements: function(path) { return []; },
  getString: function(path) { return ""; },
  startBrowserAwait: function(url, msg) { return ""; },
  putLoginInfo: function(msg) {},
  getVerificationCode: function(url) { return ""; },
  getCookie: function(url, name) { return ""; },
  timeFormat: function(ts) { return new Date(ts).toISOString(); },
  base64Encode: function(s) { return ""; },
  androidId: function() { return ""; }
};
var cookie = {
  removeCookie: function(key) {},
  getCookie: function(key) { return ""; },
  setCookie: function(key, value) {}
};
var source = {
  _var: "",
  _comment: "",
  bookSourceComment: "",
  searchUrl: "",
  bookList: "",
  getKey: function() { return ""; },
  getVariable: function() { return this._var; },
  setVariable: function(v) { this._var = String(v); },
  putLoginInfo: function(msg) {},
  getLoginInfoMap: function() { return {}; }
};
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

// InjectLegadoStubs injects Legado API stubs into a JS context.
func InjectLegadoStubs(ctx *qjs.Context) {
	// Register Go-backed ajax function for real HTTP requests
	ctx.SetFunc("__legado_ajax", func(this *qjs.This) (*qjs.Value, error) {
		args := this.Args()
		if len(args) == 0 {
			return ctx.NewString(""), nil
		}
		url := args[0].String()
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return ctx.NewString(""), nil
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ctx.NewString(""), nil
		}
		return ctx.NewString(string(body)), nil
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
