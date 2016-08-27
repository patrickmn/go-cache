package main

import (
	"flag"
	"fmt"
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
	var packageName string
	for name := range pkgs {
		packageName = name
	}
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
		},
	)
	if err != nil {
		fatal(err)
	}
}
