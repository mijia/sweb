package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path"

	"github.com/mijia/sweb/log"
	"github.com/oxtoacart/bpool"
)

const (
	kContentCharset = "; charset=UTF-8"
	kContentHtml    = "text/html"
	kContentJson    = "application/json"
)

type Delims struct {
	Left  string
	Right string
}

type Options struct {
	Directory     string
	Funcs         []template.FuncMap
	Delims        Delims
	IndentJson    bool
	UseBufPool    bool
	IsDevelopment bool
}

type TemplateSet struct {
	name     string
	fileList []string
	entry    string
	template *template.Template
}

func NewTemplateSet(name string, entry string, tFile string, otherFiles ...string) *TemplateSet {
	fileList := make([]string, 0, 1+len(otherFiles))
	fileList = append(fileList, tFile)
	for _, f := range otherFiles {
		fileList = append(fileList, f)
	}
	return &TemplateSet{
		name:     name,
		fileList: fileList,
		entry:    entry,
	}
}

type Render struct {
	opt       Options
	templates map[string]*TemplateSet
	bufPool   *bpool.BufferPool
}

func (r *Render) RawJson(w http.ResponseWriter, status int, rawData []byte) error {
	w.Header().Set("Content-Type", kContentJson+kContentCharset)
	w.WriteHeader(status)
	_, err := w.Write(rawData)
	return err
}

func (r *Render) Json(w http.ResponseWriter, status int, v interface{}) error {
	var (
		data []byte
		err  error
	)
	if r.opt.IndentJson {
		data, err = json.MarshalIndent(v, "", "  ")
		data = append(data, '\n')
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", kContentJson+kContentCharset)
	w.WriteHeader(status)
	_, err = w.Write(data)
	return err
}

func (r *Render) RawHtml(w http.ResponseWriter, status int, rawData []byte) error {
	w.Header().Set("Content-Type", kContentHtml+kContentCharset)
	w.WriteHeader(status)
	_, err := w.Write(rawData)
	return err
}

func (r *Render) Html(w http.ResponseWriter, status int, name string, binding interface{}) error {
	if r.opt.IsDevelopment {
		r.compile()
	}

	if tSet, ok := r.templates[name]; !ok {
		return fmt.Errorf("Cannot find template %q in Render", name)
	} else {
		if r.opt.UseBufPool {
			buf := r.bufPool.Get()
			if err := tSet.template.Execute(buf, binding); err != nil {
				return fmt.Errorf("Template execution error, %s", err)
			}
			w.Header().Set("Content-Type", kContentHtml+kContentCharset)
			w.WriteHeader(status)
			_, err := buf.WriteTo(w)
			r.bufPool.Put(buf)
			return err
		} else {
			out := new(bytes.Buffer)
			if err := tSet.template.Execute(out, binding); err != nil {
				return fmt.Errorf("Template execution error, %s", err)
			}
			w.Header().Set("Content-Type", kContentHtml+kContentCharset)
			w.WriteHeader(status)
			_, err := io.Copy(w, out)
			return err
		}
	}
}

func (r *Render) compile() {
	for _, ts := range r.templates {
		fileList := make([]string, len(ts.fileList))
		for i, f := range ts.fileList {
			fileList[i] = path.Join(r.opt.Directory, f)
		}
		ts.template = template.New(ts.entry)
		ts.template.Delims(r.opt.Delims.Left, r.opt.Delims.Right)
		for _, funcs := range r.opt.Funcs {
			ts.template.Funcs(funcs)
		}
		ts.template = template.Must(ts.template.ParseFiles(fileList...))
	}
	log.Debugf("Templates have been compiled, count=%d", len(r.templates))
}

func New(opt Options, tSets []*TemplateSet) *Render {
	r := &Render{
		opt:       opt,
		templates: make(map[string]*TemplateSet),
	}
	if opt.UseBufPool {
		r.bufPool = bpool.NewBufferPool(64)
	}
	for _, ts := range tSets {
		r.templates[ts.name] = ts
	}
	r.compile()
	return r
}
