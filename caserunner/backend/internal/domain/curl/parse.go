package curl

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Parse reads a curl command copied from Postman, Chrome/Firefox DevTools
// or written by hand, in bash or Windows cmd syntax. It returns a
// *ParseError when the command cannot be used.
func Parse(input string) (Result, error) {
	toks, err := tokenize(input)
	if err != nil {
		return Result{}, err
	}
	if len(toks) == 0 {
		return Result{}, syntaxError(0, "The text is empty; paste a curl command")
	}
	if name := strings.ToLower(toks[0].value); name != "curl" && name != "curl.exe" {
		return Result{}, syntaxError(toks[0].pos, "A curl command must start with curl, found %q", toks[0].value)
	}
	var p parser
	if err := p.parseArgs(toks[1:]); err != nil {
		return Result{}, err
	}
	return p.finish()
}

type parser struct {
	method   string
	head     bool // -I/--head
	url      string
	urlPos   int
	headers  []Header
	data     []string // body pieces from -d, --data-*, --json
	json     bool     // --json was used
	formBody bool     // -F or --data-urlencode: a body v1 cannot run
	insecure bool
	warnings []string
}

func (p *parser) warn(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

func (p *parser) parseArgs(toks []token) error {
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		next := func(flag string) (string, error) {
			if i+1 >= len(toks) {
				return "", syntaxError(t.pos, "%s needs a value", flag)
			}
			i++
			return toks[i].value, nil
		}
		switch {
		case t.value == "--":
			for _, rest := range toks[i+1:] {
				if err := p.setURL(rest.value, rest.pos); err != nil {
					return err
				}
			}
			return nil
		case strings.HasPrefix(t.value, "--"):
			opt, known := longOptions[t.value]
			if !known {
				p.warn("Ignored unknown flag %s", t.value)
				continue
			}
			var v string
			if opt.takesValue {
				var err error
				if v, err = next(t.value); err != nil {
					return err
				}
			}
			if err := opt.apply(p, t.value, v, t.pos); err != nil {
				return err
			}
		case len(t.value) > 1 && t.value[0] == '-':
			if err := p.parseShort(t, next); err != nil {
				return err
			}
		default:
			if err := p.setURL(t.value, t.pos); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseShort handles -X POST, -XPOST and combined flags such as -sSL or -sk.
func (p *parser) parseShort(t token, next func(string) (string, error)) error {
	letters := t.value[1:]
	for j := 0; j < len(letters); j++ {
		flag := "-" + letters[j:j+1]
		long, known := shortOptions[letters[j]]
		if !known {
			p.warn("Ignored unknown flag %s", flag)
			continue
		}
		opt := longOptions[long]
		var v string
		if opt.takesValue {
			if rest := letters[j+1:]; rest != "" {
				v = rest
			} else {
				var err error
				if v, err = next(flag); err != nil {
					return err
				}
			}
			j = len(letters) // the value used up the rest of the token
		}
		if err := opt.apply(p, flag, v, t.pos); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) setURL(raw string, pos int) error {
	if p.url != "" {
		return syntaxError(pos, "Found a second URL %q; a curl command may target only one URL", raw)
	}
	p.url, p.urlPos = raw, pos
	return nil
}

func (p *parser) addHeader(name, value string) {
	p.headers = append(p.headers, Header{Name: name, Value: value})
}

func (p *parser) hasHeader(name string) bool {
	for _, h := range p.headers {
		if strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

func (p *parser) finish() (Result, error) {
	if p.url == "" {
		return Result{}, syntaxError(0, "No URL found in the curl command")
	}
	u, err := p.normalizeURL()
	if err != nil {
		return Result{}, err
	}

	if p.json {
		if !p.hasHeader("Content-Type") {
			p.addHeader("Content-Type", "application/json")
		}
		if !p.hasHeader("Accept") {
			p.addHeader("Accept", "application/json")
		}
	}

	tpl := Template{URL: u, Headers: p.headers, Insecure: p.insecure, BodyKind: BodyNone}
	hasBody := len(p.data) > 0 || p.formBody
	if len(p.data) > 1 {
		p.warn("%d data options were joined with & into one body, as curl does", len(p.data))
	}
	tpl.BodyRaw = strings.Join(p.data, "&")
	switch {
	case p.formBody:
		tpl.BodyKind = BodyUnsupported
	case len(p.data) > 0 && json.Valid([]byte(tpl.BodyRaw)):
		tpl.BodyKind = BodyJSON
		if !p.hasHeader("Content-Type") {
			p.warn("No Content-Type header: the API may not read the body as JSON (curl itself would send application/x-www-form-urlencoded)")
		}
	case len(p.data) > 0:
		tpl.BodyKind = BodyUnsupported
	}

	switch {
	case p.method != "":
		tpl.Method = p.method
	case p.head:
		tpl.Method = "HEAD"
	case hasBody:
		tpl.Method = "POST"
	default:
		tpl.Method = "GET"
	}
	return Result{Template: tpl, Warnings: p.warnings}, nil
}

func (p *parser) normalizeURL() (string, error) {
	raw := p.url
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
		p.warn("The URL has no scheme; using %s as curl does", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", syntaxError(p.urlPos, "Invalid URL %q", p.url)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", unsupported(p.urlPos, "Only http and https URLs are supported, got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", syntaxError(p.urlPos, "The URL %q has no host", p.url)
	}
	return raw, nil
}

// option describes one curl flag; flag is the spelling the tester used.
type option struct {
	takesValue bool
	apply      func(p *parser, flag, value string, pos int) error
}

func withValue(apply func(p *parser, flag, value string, pos int) error) option {
	return option{takesValue: true, apply: apply}
}

func noValue(apply func(p *parser, flag string)) option {
	return option{apply: func(p *parser, flag, _ string, _ int) error { apply(p, flag); return nil }}
}

// ignore drops a flag that does not change the request API Case Runner sends.
func ignore(reason string) option {
	return noValue(func(p *parser, flag string) { p.warn("Ignored %s%s", flag, reason) })
}

func ignoreValue(reason string) option {
	return withValue(func(p *parser, flag, _ string, _ int) error { p.warn("Ignored %s%s", flag, reason); return nil })
}

func reject(reason string) option      { return rejectOption(reason, false) }
func rejectValue(reason string) option { return rejectOption(reason, true) }

// rejectOption refuses a flag that changes the request in a way v1 cannot reproduce.
func rejectOption(reason string, takesValue bool) option {
	return option{takesValue: takesValue, apply: func(_ *parser, flag, _ string, pos int) error {
		return unsupported(pos, "%s is not supported: %s", flag, reason)
	}}
}

func dataOption(fileAllowed bool) option {
	return withValue(func(p *parser, flag, v string, pos int) error {
		if !fileAllowed && strings.HasPrefix(v, "@") {
			return unsupported(pos, "%s %s reads the body from a file, which is not supported; paste the body itself", flag, v)
		}
		p.data = append(p.data, v)
		return nil
	})
}

func headerOption(name string) option {
	return withValue(func(p *parser, _, v string, _ int) error { p.addHeader(name, v); return nil })
}

var longOptions = map[string]option{}

func init() {
	// Request line and body.
	longOptions["--request"] = withValue(func(p *parser, _, v string, _ int) error { p.method = v; return nil })
	longOptions["--url"] = withValue(func(p *parser, _, v string, pos int) error { return p.setURL(v, pos) })
	longOptions["--head"] = noValue(func(p *parser, _ string) { p.head = true })
	longOptions["--data"] = dataOption(false)
	longOptions["--data-ascii"] = dataOption(false)
	longOptions["--data-binary"] = dataOption(false)
	longOptions["--data-raw"] = dataOption(true) // --data-raw never reads files: @ is literal
	longOptions["--json"] = withValue(func(p *parser, flag, v string, pos int) error {
		if strings.HasPrefix(v, "@") {
			return unsupported(pos, "%s %s reads the body from a file, which is not supported; paste the body itself", flag, v)
		}
		p.data, p.json = append(p.data, v), true
		return nil
	})
	formBody := withValue(func(p *parser, flag, v string, _ int) error {
		p.formBody = true
		p.data = append(p.data, v)
		p.warn("%s builds a form body; v1 supports JSON bodies only", flag)
		return nil
	})
	longOptions["--form"] = formBody
	longOptions["--form-string"] = formBody
	longOptions["--data-urlencode"] = formBody
	longOptions["--get"] = reject("it moves the body into the query string")
	longOptions["--upload-file"] = rejectValue("file uploads are not supported in v1")
	longOptions["--request-target"] = rejectValue("it changes the request path")
	longOptions["--aws-sigv4"] = rejectValue("signed requests are not supported")

	// Headers, including the flags curl turns into headers.
	longOptions["--header"] = withValue(func(p *parser, flag, v string, pos int) error {
		if strings.HasPrefix(v, "@") {
			return unsupported(pos, "%s %s reads headers from a file, which is not supported; paste the headers themselves", flag, v)
		}
		name, value, ok := strings.Cut(v, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		switch {
		case !ok && strings.HasSuffix(name, ";"): // "X-Name;" sends an empty header
			p.addHeader(strings.TrimSuffix(name, ";"), "")
		case !ok || name == "":
			p.warn("Ignored %s %q: a header must look like 'Name: value'", flag, v)
		case value == "":
			p.warn("Ignored %s %q: in curl it removes a default header", flag, v)
		default:
			p.addHeader(name, value)
		}
		return nil
	})
	longOptions["--user"] = withValue(func(p *parser, flag, v string, _ int) error {
		if !strings.Contains(v, ":") {
			p.warn("%s has no password; sending %q with an empty password", flag, v)
			v += ":"
		}
		p.addHeader("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(v)))
		return nil
	})
	longOptions["--oauth2-bearer"] = withValue(func(p *parser, _, v string, _ int) error {
		p.addHeader("Authorization", "Bearer "+v)
		return nil
	})
	longOptions["--cookie"] = withValue(func(p *parser, flag, v string, _ int) error {
		if !strings.Contains(v, "=") {
			p.warn("Ignored %s %s: reading cookies from a file is not supported", flag, v)
			return nil
		}
		p.addHeader("Cookie", v)
		return nil
	})
	longOptions["--user-agent"] = headerOption("User-Agent")
	longOptions["--referer"] = withValue(func(p *parser, _, v string, _ int) error {
		if v = strings.TrimSuffix(v, ";auto"); v != "" { // ";auto" only affects redirects
			p.addHeader("Referer", v)
		}
		return nil
	})

	// TLS.
	longOptions["--insecure"] = noValue(func(p *parser, _ string) { p.insecure = true })

	// Flags that do not change the request API Case Runner sends.
	longOptions["--location"] = ignore(": redirects are never followed, a 3xx is reported as the result")
	longOptions["--location-trusted"] = longOptions["--location"]
	for _, f := range []string{
		"--compressed", "--silent", "--show-error", "--verbose", "--include", "--fail", "--fail-with-body",
		"--globoff", "--no-buffer", "--http1.0", "--http1.1", "--http2", "--http2-prior-knowledge", "--http3",
		"--ipv4", "--ipv6", "--progress-bar", "--remote-name", "--netrc", "--path-as-is", "--raw",
		"--tr-encoding", "--no-keepalive", "--no-progress-meter", "--styled-output", "--no-styled-output",
	} {
		longOptions[f] = ignore("")
	}
	longOptions["--max-time"] = ignoreValue(": the run's timeout applies")
	longOptions["--connect-timeout"] = longOptions["--max-time"]
	longOptions["--retry"] = ignoreValue(": API Case Runner never retries a request")
	longOptions["--proxy"] = ignoreValue(": requests are sent directly")
	longOptions["--resolve"] = ignoreValue(": the host is resolved normally")
	longOptions["--connect-to"] = longOptions["--resolve"]
	for _, f := range []string{
		"--output", "--cookie-jar", "--dump-header", "--write-out", "--proxy-user", "--proxy-header", "--noproxy",
		"--retry-delay", "--retry-max-time", "--cacert", "--capath", "--cert", "--key", "--cert-type", "--key-type",
		"--limit-rate", "--max-redirs", "--interface", "--range", "--config", "--trace", "--trace-ascii", "--stderr",
		"--unix-socket", "--time-cond", "--continue-at", "--speed-time", "--speed-limit", "--quote", "--ftp-port",
		"--keepalive-time", "--local-port", "--max-filesize", "--expect100-timeout", "--dns-servers", "--tls-max",
		"--ciphers", "--variable", "--etag-save", "--etag-compare", "--happy-eyeballs-timeout-ms",
	} {
		longOptions[f] = ignoreValue("")
	}
}

// shortOptions maps single-letter flags to their long form.
var shortOptions = map[byte]string{
	'X': "--request", 'H': "--header", 'd': "--data", 'u': "--user", 'k': "--insecure", 'I': "--head",
	'b': "--cookie", 'A': "--user-agent", 'e': "--referer", 'F': "--form", 'G': "--get", 'T': "--upload-file",
	'L': "--location", 's': "--silent", 'S': "--show-error", 'v': "--verbose", 'i': "--include", 'f': "--fail",
	'g': "--globoff", 'N': "--no-buffer", '0': "--http1.0", '4': "--ipv4", '6': "--ipv6", '#': "--progress-bar",
	'O': "--remote-name", 'n': "--netrc", 'm': "--max-time", 'x': "--proxy", 'o': "--output", 'c': "--cookie-jar",
	'D': "--dump-header", 'w': "--write-out", 'U': "--proxy-user", 'E': "--cert", 'K': "--config", 'r': "--range",
	'z': "--time-cond", 'C': "--continue-at", 'y': "--speed-time", 'Y': "--speed-limit", 'Q': "--quote", 'P': "--ftp-port",
}
