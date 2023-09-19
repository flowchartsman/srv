package ui

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"strings"
	"time"
)

type TmplFS struct {
	base  fs.FS
	data  any
	tmpls *template.Template
}

func NewTmplFS(baseFS fs.FS, data any) (fs.FS, error) {
	tmpl, err := template.New("").ParseFS(baseFS, "*.tmpl")
	if err != nil {
		return nil, err
	}
	if tmpl.DefinedTemplates() == "" {
		return nil, errors.New("no templates found")
	}
	return &TmplFS{baseFS, data, tmpl}, nil
}

func (t *TmplFS) Open(name string) (fs.File, error) {
	tmpl := t.tmpls.Lookup(name + ".tmpl")
	if tmpl != nil {
		return newTmplFile(name+".tmpl", tmpl, t.data)
	}
	return t.base.Open(name)
}

type tmplFile struct {
	name string
	buf  *bytes.Reader
}

func newTmplFile(name string, tmpl *template.Template, data any) (*tmplFile, error) {
	if strings.HasSuffix(name, ".tmpl") {
		name = strings.TrimSuffix(name, ".tmpl")
	}
	buf := &bytes.Buffer{}
	if err := tmpl.Execute(buf, data); err != nil {
		return nil, err
	}
	return &tmplFile{
		name: name,
		buf:  bytes.NewReader(buf.Bytes()),
	}, nil
}

func (tf *tmplFile) Close() error { return nil }

func (tf *tmplFile) Read(bs []byte) (int, error) {
	return tf.buf.Read(bs)
}

func (tf *tmplFile) Stat() (fs.FileInfo, error) {
	return &tmplInfo{tf.name, tf.buf.Size()}, nil
}

type tmplInfo struct {
	name string
	size int64
}

func (t *tmplInfo) Name() string {
	return t.name
}

func (t *tmplInfo) Size() int64 {
	return t.size
}

func (t *tmplInfo) Mode() fs.FileMode {
	return 0o0666
}

func (t *tmplInfo) ModTime() time.Time {
	return time.Time{}.Add(time.Second)
}

func (t *tmplInfo) IsDir() bool {
	return false
}

func (t *tmplInfo) Sys() any {
	return nil
}
