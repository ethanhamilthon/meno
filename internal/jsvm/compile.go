package jsvm

import (
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
)

// codeKind says how a piece of tool code is run.
type codeKind int

const (
	kindScript codeKind = iota // plain script: the value is the last expression
	kindSync                   // wrapped in a function: top-level return is allowed
	kindAsync                  // wrapped in an async function: top-level await too
)

// compiled is tool code ready to run in a runtime.
type compiled struct {
	prog *goja.Program
	kind codeKind
}

// compile turns tool code into a program. Plain scripts run as they are, which
// gives the completion value of the last expression. Code that is not a valid
// script because it uses a top-level return or await is parsed inside a
// function instead, and its last expression statement becomes an explicit
// return. The decision comes from the parser, never from searching the text.
func compile(code string) (compiled, error) {
	prog, scriptErr := goja.Compile("tool.js", code, false)
	if scriptErr == nil {
		return compiled{prog, kindScript}, nil
	}
	var asyncErr error
	for _, kind := range []codeKind{kindSync, kindAsync} {
		src, err := wrapBody(code, kind)
		if err != nil {
			asyncErr = err
			continue
		}
		prog, err := goja.Compile("tool.js", src, false)
		if err != nil {
			asyncErr = err
			continue
		}
		return compiled{prog, kind}, nil
	}
	// Report the error of the most permissive reading: it points at the real
	// mistake rather than at a return or await that is fine in a function.
	if asyncErr == nil {
		asyncErr = scriptErr
	}
	return compiled{}, asyncErr
}

func wrapPrefix(kind codeKind) string {
	if kind == kindAsync {
		return "async function __meno(){"
	}
	return "function __meno(){"
}

// wrapBody parses code as the body of a function and returns the source of an
// immediately invoked function whose result is the code's last expression
// statement (or the explicit return of the code).
func wrapBody(code string, kind codeKind) (string, error) {
	prefix := wrapPrefix(kind)
	probe := prefix + code + "\n}"
	ast0, err := goja.Parse("tool.js", probe)
	if err != nil {
		return "", err
	}
	body := code
	if fn := firstFunction(ast0); fn != nil && len(fn.Body.List) > 0 {
		if es, ok := fn.Body.List[len(fn.Body.List)-1].(*ast.ExpressionStatement); ok {
			// Idx is 1-based within the probe source.
			at := int(es.Expression.Idx0()) - 1 - len(prefix)
			if at >= 0 && at <= len(code) {
				body = code[:at] + "return " + code[at:]
			}
		}
	}
	return "(" + strings.Replace(prefix, "function __meno", "function", 1) + body + "\n})()", nil
}

func firstFunction(p *ast.Program) *ast.FunctionLiteral {
	for _, st := range p.Body {
		if d, ok := st.(*ast.FunctionDeclaration); ok {
			return d.Function
		}
	}
	return nil
}
