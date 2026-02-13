package middleware

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/gowool/keratin/internal"
)

type Skipper func(*http.Request) bool

func ChainSkipper(skippers ...Skipper) Skipper {
	return func(r *http.Request) bool {
		for _, skipper := range skippers {
			if skipper(r) {
				return true
			}
		}
		return false
	}
}

func PrefixPathSkipper(prefixes ...string) Skipper {
	prefixes = internal.Map(prefixes, strings.ToLower)
	return func(req *http.Request) bool {
		p := strings.ToLower(req.URL.Path)
		m := strings.ToLower(req.Method)
		for _, prefix := range prefixes {
			if prefix, ok := CheckMethod(m, prefix); ok && strings.HasPrefix(p, prefix) {
				return true
			}
		}
		return false
	}
}

func SuffixPathSkipper(suffixes ...string) Skipper {
	suffixes = internal.Map(suffixes, strings.ToLower)
	return func(req *http.Request) bool {
		p := strings.ToLower(req.URL.Path)
		m := strings.ToLower(req.Method)
		for _, suffix := range suffixes {
			if suffix, ok := CheckMethod(m, suffix); ok && strings.HasSuffix(p, suffix) {
				return true
			}
		}
		return false
	}
}

func EqualPathSkipper(paths ...string) Skipper {
	return func(req *http.Request) bool {
		for _, path := range paths {
			if path, ok := CheckMethod(req.Method, path); ok && strings.EqualFold(req.URL.Path, path) {
				return true
			}
		}
		return false
	}
}

func CheckMethod(method, pattern string) (string, bool) {
	if index := strings.IndexRune(pattern, ' '); index > 0 {
		if method == pattern[:index] {
			return strings.TrimSpace(pattern[index+1:]), true
		}
		return "", false
	}
	return pattern, true
}

// ExpressionSkipper creates a Skipper function that evaluates expressions against an environment generated from requests.
//
// Documentation can be found here: https://expr-lang.org/
func ExpressionSkipper[Env any](fn func(*http.Request) Env, expressions ...string) Skipper {
	zero := reflect.Zero(reflect.TypeFor[Env]()).Interface()
	programs := make([]*vm.Program, len(expressions))
	for i, expression := range expressions {
		program, err := expr.Compile(expression, expr.Env(zero), expr.AsBool())
		if err != nil {
			continue
		}
		programs[i] = program
	}

	return func(r *http.Request) bool {
		env := fn(r)

		for _, program := range programs {
			if ok, err := expr.Run(program, env); err == nil && ok.(bool) {
				return true
			}
		}
		return false
	}
}
