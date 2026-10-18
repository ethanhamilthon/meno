package jsvm

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

func digestBytes(data []byte, algo string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(algo)) {
	case "", "sha256", "sha-256":
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]), nil
	case "sha1", "sha-1":
		sum := sha1.Sum(data)
		return hex.EncodeToString(sum[:]), nil
	case "md5":
		sum := md5.Sum(data)
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("hash: unsupported algorithm %q (allowed: sha256, sha1, md5)", algo)
	}
}

func decodeBase64(s string, urlSafe bool) (string, error) {
	s = strings.TrimSpace(s)
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	if urlSafe {
		encodings = []*base64.Encoding{base64.URLEncoding, base64.RawURLEncoding, base64.StdEncoding, base64.RawStdEncoding}
	}
	var lastErr error
	for _, enc := range encodings {
		b, err := enc.DecodeString(s)
		if err == nil {
			return string(b), nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("base64.decode: %w", lastErr)
}

func (v *VM) bindStdlib() {
	rt := v.rt
	throw := func(err error) {
		if err != nil {
			panic(rt.NewGoError(err))
		}
	}

	// hash(text, algo?) and hash.sha256 / hash.sha1 / hash.md5 / hash.file
	hashFn := rt.ToValue(func(c goja.FunctionCall) goja.Value {
		s := strArg(rt, c.Arguments, 0, "text")
		algo := "sha256"
		if len(c.Arguments) > 1 && !goja.IsUndefined(c.Arguments[1]) && !goja.IsNull(c.Arguments[1]) {
			algo = strArg(rt, c.Arguments, 1, "algorithm")
		}
		out, err := digestBytes([]byte(s), algo)
		throw(err)
		return rt.ToValue(out)
	}).ToObject(rt)
	_ = hashFn.Set("sha256", func(s string) string {
		out, _ := digestBytes([]byte(s), "sha256")
		return out
	})
	_ = hashFn.Set("sha1", func(s string) string {
		out, _ := digestBytes([]byte(s), "sha1")
		return out
	})
	_ = hashFn.Set("md5", func(s string) string {
		out, _ := digestBytes([]byte(s), "md5")
		return out
	})
	_ = hashFn.Set("file", func(c goja.FunctionCall) goja.Value {
		p := strArg(rt, c.Arguments, 0, "path")
		algo := "sha256"
		if len(c.Arguments) > 1 && !goja.IsUndefined(c.Arguments[1]) && !goja.IsNull(c.Arguments[1]) {
			algo = strArg(rt, c.Arguments, 1, "algorithm")
		}
		full, err := (FS{Root: v.Root()}).path(p)
		throw(err)
		b, err := os.ReadFile(full)
		throw(err)
		out, err := digestBytes(b, algo)
		throw(err)
		return rt.ToValue(out)
	})
	_ = rt.Set("hash", hashFn)

	// path utilities
	pathObj := rt.NewObject()
	joinFn := func(c goja.FunctionCall) goja.Value {
		parts := make([]string, 0, len(c.Arguments))
		for i := range c.Arguments {
			parts = append(parts, strArg(rt, c.Arguments, i, "path"))
		}
		if len(parts) == 0 {
			return rt.ToValue(".")
		}
		return rt.ToValue(filepath.Join(parts...))
	}
	dirFn := func(p string) string { return filepath.Dir(p) }
	baseFn := func(c goja.FunctionCall) goja.Value {
		p := strArg(rt, c.Arguments, 0, "path")
		b := filepath.Base(p)
		if len(c.Arguments) > 1 && !goja.IsUndefined(c.Arguments[1]) && !goja.IsNull(c.Arguments[1]) {
			ext := strArg(rt, c.Arguments, 1, "ext")
			b = strings.TrimSuffix(b, ext)
		}
		return rt.ToValue(b)
	}
	extFn := func(p string) string { return filepath.Ext(p) }
	resolveFn := func(c goja.FunctionCall) goja.Value {
		cur := v.Root()
		for i := range c.Arguments {
			seg := strArg(rt, c.Arguments, i, "path")
			if seg == "" {
				continue
			}
			resolved, err := (FS{Root: cur}).path(seg)
			throw(err)
			cur = resolved
		}
		return rt.ToValue(filepath.Clean(cur))
	}
	relFn := func(c goja.FunctionCall) goja.Value {
		from := v.Root()
		var to string
		if len(c.Arguments) == 1 {
			to = strArg(rt, c.Arguments, 0, "to")
		} else {
			from = strArg(rt, c.Arguments, 0, "from")
			to = strArg(rt, c.Arguments, 1, "to")
		}
		fromAbs, err := (FS{Root: v.Root()}).path(from)
		throw(err)
		toAbs, err := (FS{Root: v.Root()}).path(to)
		throw(err)
		r, err := filepath.Rel(fromAbs, toAbs)
		throw(err)
		return rt.ToValue(r)
	}
	cleanFn := func(p string) string { return filepath.Clean(p) }
	isAbsFn := func(p string) bool { return filepath.IsAbs(p) }
	parseFn := func(p string) map[string]any {
		dir := filepath.Dir(p)
		base := filepath.Base(p)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		root := ""
		if filepath.IsAbs(p) {
			root = string(filepath.Separator)
		}
		return map[string]any{"root": root, "dir": dir, "base": base, "ext": ext, "name": name}
	}
	_ = pathObj.Set("join", joinFn)
	_ = pathObj.Set("dirname", dirFn)
	_ = pathObj.Set("dir", dirFn)
	_ = pathObj.Set("basename", baseFn)
	_ = pathObj.Set("base", baseFn)
	_ = pathObj.Set("extname", extFn)
	_ = pathObj.Set("ext", extFn)
	_ = pathObj.Set("resolve", resolveFn)
	_ = pathObj.Set("relative", relFn)
	_ = pathObj.Set("rel", relFn)
	_ = pathObj.Set("clean", cleanFn)
	_ = pathObj.Set("normalize", cleanFn)
	_ = pathObj.Set("isAbsolute", isAbsFn)
	_ = pathObj.Set("isAbs", isAbsFn)
	_ = pathObj.Set("parse", parseFn)
	_ = pathObj.Set("sep", string(filepath.Separator))
	_ = rt.Set("path", pathObj)

	// base64 with UTF-8
	b64Obj := rt.NewObject()
	encStd := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	decStd := func(s string) string {
		out, err := decodeBase64(s, false)
		throw(err)
		return out
	}
	encURL := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	decURL := func(s string) string {
		out, err := decodeBase64(s, true)
		throw(err)
		return out
	}
	_ = b64Obj.Set("encode", encStd)
	_ = b64Obj.Set("decode", decStd)
	_ = b64Obj.Set("encodeURL", encURL)
	_ = b64Obj.Set("decodeURL", decURL)
	_ = rt.Set("base64", b64Obj)
	_ = rt.Set("btoa", encStd)
	_ = rt.Set("atob", decStd)
}
