package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"sort"
)

type importKey struct {
	name string
	path string
}

func main() {
	outPath := flag.String("out", "", "output file path")
	flag.Parse()
	inPaths := flag.Args()

	if *outPath == "" || len(inPaths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: collapse_go_package_files -out <output.go> <input1.go> <input2.go> ...")
		os.Exit(2)
	}

	fset := token.NewFileSet()
	importsByKey := make(map[importKey]struct{})
	decls := make([]ast.Decl, 0, 512)
	packageName := ""

	for _, inPath := range inPaths {
		parsedFile, parseErr := parser.ParseFile(fset, inPath, nil, parser.ParseComments)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", inPath, parseErr)
			os.Exit(1)
		}

		if packageName == "" {
			packageName = parsedFile.Name.Name
		}
		if parsedFile.Name.Name != packageName {
			fmt.Fprintf(os.Stderr, "mixed package names: %s has %s, expected %s\n", inPath, parsedFile.Name.Name, packageName)
			os.Exit(1)
		}

		for _, importSpec := range parsedFile.Imports {
			importName := ""
			if importSpec.Name != nil {
				importName = importSpec.Name.Name
			}
			importsByKey[importKey{name: importName, path: importSpec.Path.Value}] = struct{}{}
		}

		for _, declaration := range parsedFile.Decls {
			genDecl, isGenDecl := declaration.(*ast.GenDecl)
			if isGenDecl && genDecl.Tok == token.IMPORT {
				continue
			}
			decls = append(decls, declaration)
		}
	}

	if packageName == "" {
		fmt.Fprintln(os.Stderr, "no package declarations found")
		os.Exit(1)
	}

	importKeys := make([]importKey, 0, len(importsByKey))
	for key := range importsByKey {
		importKeys = append(importKeys, key)
	}
	sort.Slice(importKeys, func(i, j int) bool {
		if importKeys[i].path == importKeys[j].path {
			return importKeys[i].name < importKeys[j].name
		}
		return importKeys[i].path < importKeys[j].path
	})

	combinedDecls := make([]ast.Decl, 0, len(decls)+1)
	if len(importKeys) > 0 {
		importSpecs := make([]ast.Spec, 0, len(importKeys))
		for _, key := range importKeys {
			importSpec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: key.path}}
			if key.name != "" {
				importSpec.Name = ast.NewIdent(key.name)
			}
			importSpecs = append(importSpecs, importSpec)
		}
		combinedDecls = append(combinedDecls, &ast.GenDecl{Tok: token.IMPORT, Specs: importSpecs})
	}
	combinedDecls = append(combinedDecls, decls...)

	combinedFile := &ast.File{Name: ast.NewIdent(packageName), Decls: combinedDecls}

	var out bytes.Buffer
	if err := format.Node(&out, fset, combinedFile); err != nil {
		fmt.Fprintf(os.Stderr, "format node: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*outPath, out.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *outPath, err)
		os.Exit(1)
	}
}
