package util

import (
	"html/template"
	"io"
)

type TemplateProxy interface {
	ExecuteTemplate(wr io.Writer, name string, data interface{}) error
}

type DynamicTemplateProxy struct {
	Glob string
}

func (proxy DynamicTemplateProxy) ExecuteTemplate(
	wr io.Writer,
	name string,
	data interface{},
) error {
	tmpl, err := template.ParseGlob(proxy.Glob)
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(wr, name, data)
}

type StaticTemplateProxy struct {
	Template *template.Template
}

func (proxy StaticTemplateProxy) ExecuteTemplate(
	wr io.Writer,
	name string,
	data interface{},
) error {
	return proxy.Template.ExecuteTemplate(wr, name, data)
}
