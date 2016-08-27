package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"text/template"
)

func fatal(v ...interface{}) {
	fmt.Fprintln(os.Stderr, v...)
	os.Exit(1)
}

func packageDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("No caller information")
	}
	return path.Dir(filename)
}

// find value of ident 'grammar' in GenDecl.
func findInGenDecl(genDecl *ast.GenDecl, grammarName string) string {
	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.TypeSpec)
		if ok {
			// type ident
			ident, ok := valueSpec.Type.(*ast.Ident)
			if ok {
				return ident.Name
			}
		}
	}
	return ""
}

func findInDecl(decl ast.Decl, grammarName string) string {
	genDecl, ok := decl.(*ast.GenDecl)
	if ok {
		g := findInGenDecl(genDecl, grammarName)
		if g != "" {
			return g
		}
	}
	return ""
}

// zeroValue returns literal zero value.
func zeroValue(s string) string {
	// TODO: support func type.
	switch s {
	case "string":
		return "\"\""
	case "int", "uint", "int64", "uint64", "uint32", "int32", "int16",
		"uint16", "int8", "uint8", "byte", "rune", "float64", "float32",
		"complex64", "complex32", "uintptr":
		return "0"
	case "slice":
		return "nil"
	default:
		if s[0] == '*' { // Pointer
			return "nil"
		}
		return s + "{}"
	}
}

// TODO: support more builtin types
func builtin(s string) bool {
	switch s {
	case "string":
		return true
	}
	return false
}

func main() {
	keyType := flag.String("k", "", "key type")
	valueType := flag.String("v", "", "value type")
	flag.Parse()
	if *keyType == "" {
		fatal("key empty")
	}
	if *valueType == "" {
		fatal("value empty")
	}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		fatal(err)
	}
	packageName := "main"
	typeName := ""
	for name, pkg := range pkgs {
		packageName = name
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				typeName = findInDecl(decl, *valueType)
			}
		}
	}
	if typeName == "" && !builtin(*valueType) {
		fatal(fmt.Errorf("found no definition of %s in files\n", *valueType))
	}
	if typeName == "" {
		typeName = *valueType
	}
	zeroTypeValue := zeroValue(typeName)
	f, err := os.OpenFile(fmt.Sprintf("%s2%s_cachemap.go", *keyType, *valueType), os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	tpl, err := template.New("cache.tmpl").ParseFiles(filepath.Join(packageDir(), "cache.tmpl"))
	if err != nil {
		fatal(err)
	}
	err = tpl.Execute(
		f,
		map[string]string{
			"ValueType":   *valueType,
			"PackageName": packageName,
			"Cache":       fmt.Sprintf("String2%sCache", *valueType),
			"ZeroValue":   zeroTypeValue,
		},
	)
	if err != nil {
		fatal(err)
	}
}
