// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"golang.org/x/tools/imports"
)

const DEFAULT_TEMPLATE = `
// DO NOT EDIT, auto generated by struct2interface

package {{.Package}}

type {{.Name}} interface {
	{{.Content}}
}
`

func functionDef(fun *ast.FuncDecl, fset *token.FileSet) string {
	name := fun.Name.Name
	params := make([]string, 0)
	for _, p := range fun.Type.Params.List {
		var typeNameBuf bytes.Buffer
		err := printer.Fprint(&typeNameBuf, fset, p.Type)
		if err != nil {
			log.Fatalf("failed printing %s", err)
		}
		names := make([]string, 0)
		for _, name := range p.Names {
			names = append(names, name.Name)
		}
		params = append(params, fmt.Sprintf("%s %s", strings.Join(names, ","), typeNameBuf.String()))
	}
	returns := make([]string, 0)
	if fun.Type.Results != nil {
		for _, r := range fun.Type.Results.List {
			var typeNameBuf bytes.Buffer
			err := printer.Fprint(&typeNameBuf, fset, r.Type)
			if err != nil {
				log.Fatalf("failed printing %s", err)
			}

			returns = append(returns, typeNameBuf.String())
		}
	}
	returnString := ""
	if len(returns) == 1 {
		returnString = returns[0]
	} else if len(returns) > 1 {
		returnString = fmt.Sprintf("(%s)", strings.Join(returns, ", "))
	}
	return fmt.Sprintf("%s (%s) %v", name, strings.Join(params, ", "), returnString)
}

func generateInterface(folder, outputFile, pkgName, structName, ifName, outputTemplate string) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, folder, nil, parser.AllErrors)
	if err != nil {
		log.Fatalf("Unable to parse %s folder", folder)
	}
	var appPkg *ast.Package
	for _, pkg := range pkgs {
		if pkg.Name == pkgName {
			appPkg = pkg
			break
		}
	}
	if appPkg == nil {
		log.Fatalf("Unable to find package %s", pkgName)
	}

	funcs := make([]string, 0)
	for _, file := range appPkg.Files {
		log.Printf("parsing %s\n", fset.File(file.Pos()).Name())
		if fset.File(file.Pos()).Name() == outputFile {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if fun, ok := n.(*ast.FuncDecl); ok {
				if fun.Recv != nil {
					if fun.Name.IsExported() {
						if fun.Recv != nil && len(fun.Recv.List) == 1 {
							if r, rok := fun.Recv.List[0].Type.(*ast.StarExpr); rok && r.X.(*ast.Ident).Name == structName {
								funcs = append(funcs, functionDef(fun, fset))
							}
						}
					}
				}

			}
			return true
		})
	}
	sort.Strings(funcs)
	out := bytes.NewBufferString("")

	t := template.Must(template.New("").Parse(outputTemplate))
	err = t.Execute(out, map[string]interface{}{
		"Content": strings.Join(funcs, "\n"),
		"Name":    ifName,
		"Package": pkgName,
	})
	if err != nil {
		log.Panic(err)
	}
	os.Remove(outputFile)
	formatted, err := imports.Process(outputFile, out.Bytes(), &imports.Options{Comments: true})
	if err != nil {
		log.Panic(err)
	}
	err = ioutil.WriteFile(outputFile, formatted, 0644)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Written %s successfully", outputFile)
}

func main() {

	var folder, outputFile, pkgName, structName, ifName, outputTemplateFile string
	outputTemplate := DEFAULT_TEMPLATE

	var rootCmd = &cobra.Command{
		Use:   "struct2interface",
		Short: "Extract an interface from a Golang struct",
		Run: func(cmd *cobra.Command, args []string) {

			if outputTemplateFile != "" {
				d, err := ioutil.ReadFile(outputTemplateFile)
				if err != nil {
					log.Panic(err)
				}
				outputTemplate = string(d)
			}

			generateInterface(folder, outputFile, pkgName, structName, ifName, outputTemplate)
		},
	}

	rootCmd.Flags().StringVarP(&folder, "folder", "f", "", "Path to the package in which the struct resides")
	_ = rootCmd.MarkFlagRequired("folder")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Path to output file (will be overwritten)")
	_ = rootCmd.MarkFlagRequired("output")
	rootCmd.Flags().StringVarP(&pkgName, "package", "p", "", "Name of the package in which the struct resides")
	_ = rootCmd.MarkFlagRequired("package")
	rootCmd.Flags().StringVarP(&structName, "struct", "s", "", "Name of the input struct")
	_ = rootCmd.MarkFlagRequired("struct")
	rootCmd.Flags().StringVarP(&ifName, "interface", "i", "", "Name of the output interface")
	_ = rootCmd.MarkFlagRequired("interface")
	rootCmd.Flags().StringVarP(&outputTemplateFile, "template", "t", "", "Path to a Go template file to use for writing the resulting interface")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
