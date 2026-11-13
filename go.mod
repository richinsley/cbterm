module github.com/calobozan/cbterm

go 1.22

require github.com/creack/pty v1.1.24

require github.com/webui-dev/go-webui/v2 v2.5.3

// cbterm: local patch of go-webui — disables WKProcessPool process suspension
// (WKWebView WebContent process is suspended by WebKit when the window is not
// visible, which kills all JS/WebSocket traffic and breaks Script()).
// See third_party/go-webui/webui/src/webview/wkwebview.m.
replace github.com/webui-dev/go-webui/v2 => ./third_party/go-webui
