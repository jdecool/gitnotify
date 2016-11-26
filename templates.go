package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"strings"
)

const pathPartialTemplate = "tmpl/partials/"
const pathTemplate = "tmpl/"

// SimpleTemplate ...
type SimpleTemplate struct {
	prefix      string
	partialsDir string
	t           *template.Template
}

var templates *SimpleTemplate

func init() {
	reloadTemplates()
}

func reloadTemplates() {
	// Templates with functions available to them
	templates = &SimpleTemplate{
		"tmpl/",
		"tmpl/partials/",
		template.New("").Funcs(templateMap),
	}
	load()
	loadPartials()
}

func displayText(hc *httpContext, w io.Writer, text string) {
	page := newPage(hc, text, text)
	displayPage(w, "text", page)
}

// page path relative to 'tmpl', example "settings"
func displayPage(w io.Writer, page string, data interface{}) {
	// reload only in dev environments
	reloadTemplates()

	tv := templates.t.Lookup(pathTemplate + page)
	tv.Execute(w, data)
}

func load() {
	loadFilesFromDir(templates.prefix, pathTemplate)
}

func loadPartials() {
	loadFilesFromDir(templates.partialsDir, pathPartialTemplate)
}

func loadFilesFromDir(dir, pathDir string) {
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		name := dir + fi.Name()
		tmplName := strings.Replace(fi.Name(), ".html", "", 1)

		b, err := ioutil.ReadFile(name)
		_, err = templates.t.New(pathDir + tmplName).Parse(string(b))

		if err != nil {
			fmt.Println(err)
		}
	}
}