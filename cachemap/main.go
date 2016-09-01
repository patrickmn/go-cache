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
	"strings"
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

// TODO: parse type if type is also not builtin type.
// find literal value of type.
func findInGenDecl(genDecl *ast.GenDecl, valueName string) string {
	for _, spec := range genDecl.Specs {
		valueSpec, ok := spec.(*ast.TypeSpec)
		if ok {
			if ok && valueSpec.Name.Name == valueName {
				indent, ok := valueSpec.Type.(*ast.Ident)
				if ok {
					return indent.Name
				} else {
					// For other types like StructType
					return valueName
				}
			}
		}
	}
	return ""
}

func findInDecl(decl ast.Decl, valueName string) string {
	genDecl, ok := decl.(*ast.GenDecl)
	if ok {
		g := findInGenDecl(genDecl, valueName)
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
	case "bool":
		return "false"
	case "string":
		return "\"\""
	case "int", "uint", "int64", "uint64", "uint32", "int32", "int16",
		"uint16", "int8", "uint8", "byte", "rune", "float64", "float32",
		"complex64", "complex32", "uintptr":
		return "0"
	default:
		if s[0] == '*' || // Pointer
			strings.Index(s, "map") == 0 || // map
			strings.Index(s, "chan") == 0 || // chan
			strings.Index(s, "[]") == 0 || // slice
			strings.Index(s, "func") == 0 { // func
			return "nil"
		}
		return s + "{}"
	}
}

var builtinTypes = []string{
	"bool",
	"string",

	"int", "int8", "int16", "int32", "int64", // numbericType
	"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
	"float32", "float64",
	"complex64", "complex128",
	"byte",
	"rune",
}

func isNumberic(s string) bool {
	for _, v := range builtinTypes[2:] { // 2 is beginning of numberic types in builtinTypes.
		if v == s {
			return true
		}
	}
	return false
}

func isBuiltin(s string) bool {
	for _, v := range builtinTypes {
		if v == s {
			return true
		}
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
FIND:
	for name, pkg := range pkgs {
		packageName = name
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				typeName = findInDecl(decl, *valueType)
				if typeName != "" {
					break FIND
				}
			}
		}
	}
	if typeName == "" && !isBuiltin(*valueType) {
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
	if !isBuiltin(*valueType) {
		*valueType = strings.Title(*valueType)
	}
	err = tpl.Execute(
		f,
		map[string]interface{}{
			"ValueType":   *valueType,
			"RealType":    typeName,
			"PackageName": packageName,
			"Cache":       fmt.Sprintf("String2%sCache", *valueType),
			"ZeroValue":   zeroTypeValue,
			"IsNumberic":  isNumberic,
		},
	)
	if err != nil {
		fatal(err)
	}
	constFile, err := os.OpenFile(fmt.Sprintf("cachemap_const.go"), os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		fatal(err)
	}
	defer constFile.Close()
	constTpl, err := template.New("const.tmpl").ParseFiles(filepath.Join(packageDir(), "const.tmpl"))
	if err != nil {
		fatal(err)
	}
	err = constTpl.Execute(constFile,
		map[string]interface{}{
			"PackageName": packageName,
		})
	if err != nil {
		fatal(err)
	}
}
