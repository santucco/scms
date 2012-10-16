package scms

import (
	"http"
	"template"
	"os"
	"json"
	"io"
	"bytes"
	"strings"
	"archive/zip"
	"appengine"
	"appengine/datastore"
	"appengine/user"
)

type Config struct {
	Default *datastore.Key
}

type Page struct {
	Name     string
	Base     string
	Template string
}

var pagesTemplate = template.Must(template.New("pages").Funcs(funcMap).Parse(
	`
<html>
<body>
<a href="/">Main</a><br>
<a href="/editor">Editor</a><br>
<a href="/logout">Logout</a><br>
{{$files := .Get "$Files" "Name" "" 0 0}}
{{$cursor :=.Get "$Pages" "Name" "" 0 0}}
{{$cfg := .GetByKeyFields "$Config" "config" 0 ""}}
{{$default := $cfg.Data.Default}}
<form action="/editor/pages" method="post">
	<fieldset>
		<legend>Default</legend>
		<select name="default">
			{{range $cursor}}
				<option value={{.Key.Encode}} {{if  $default.Eq .Key}}selected{{end}}>{{.Data.Name}}
			{{end}}
		</select>
		<input type="submit" value="Submit">
		<input type="reset" value="Reset">
	</fieldset>
</form>
<form action="/editor/pages" method="post" enctype="multipart/form-data">
	<fieldset>
		<legend>New page</legend>
		<label>Name of page:<br><input type="text" name="name" value=""></label><br>
		<legend>File with HTML-template:</legend>
		<select name="file">
			{{range $files }}
				<option value={{.Data.Name}}>{{.Data.Name}}
			{{end}}
		</select>	
		<br>
		<legend>File with base template:</legend>
		<select name="base">
			{{range $files }}
				<option value={{.Data.Name}}>{{.Data.Name}}
			{{end}}
		</select>
		<br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
<form action="/editor/pages?action=upload" method="post" enctype="multipart/form-data">
	<fieldset>
	{{if $cursor.Len}}
		<a href=/editor/pages.zip>Download all pages</a><br>
	{{end}}
		<label>Upload pages: <input type="file" name="file" value=""></label><br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
{{range $cursor}}
<form action="/editor/pages?id={{.Key.Encode}}" method="post" enctype="multipart/form-data">
	<fieldset>
		<legend>Page "{{.Data.Name}}"</legend>
		<legend>File with HTML-template:</legend>
		{{$base := .Data.Template}}
		<select name="file">
			{{range $files }}
				<option value={{.Data.Name}} {{if EqualString .Data.Name $base}}selected{{end}}>{{.Data.Name}}
			{{end}}
		</select>	
		<br>
		<legend>File with base template:</legend>
		{{$base := .Data.Base}}
		<select name="base">
			{{range $files }}
				<option value={{.Data.Name}} {{if EqualString .Data.Name $base}}selected{{end}}>{{.Data.Name}}
			{{end}}
		</select>
		<br>
		<input type="submit" value="Submit">
		<input type="reset" value="Reset">
		<input type="button" value="Delete">
	</fieldset>
</form>
{{end}}
</body>
</html>
`))

func pagesHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	var key *datastore.Key
	if id := r.URL.Query().Get("id"); len(id) != 0 {
		if k, err := datastore.DecodeKey(id); err != nil {
			error(c, w, err)
			return
		} else {
			key = k
		}
	}
	if r.Method == "GET" {
		if key != nil {
			if err := getTemplate(w, c, r, key); err != nil {
				error(c, w, err)
			}
			return
		}
		var data Context
		data.ctx = c
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := pagesTemplate.Execute(w, &data); err != nil {
			error(c, w, err)
		}
		return
	} else if r.Method != "POST" {
		error404(w, r)
		return
	}
	if r.FormValue("action") == "upload" {
		file, _, err := r.FormFile("file")
		if err != nil {
			error(c, w, err)
			c.Errorf("can't get an archive with imported pages: %q", err)
			return
		}
		if n, err := file.Seek(0, os.SEEK_END); err != nil {
			error(c, w, err)
			return
		} else if err := importPages(c, file, n); err != nil {
			error(c, w, err)
			return
		}
	} else if def := r.FormValue("default"); len(def) != 0 {
		if err := setDefault(c, r, def); err != nil {
			error(c, w, err)
			return
		}
	} else {
		if key == nil {
			if err := newPage(c, r); err != nil {
				error(c, w, err)
				return
			}
		} else if err := editPage(c, r, key); err != nil {
			error(c, w, err)
			return
		}
	}
	createHandlers(c)
	http.Redirect(w, r, r.URL.Path, http.StatusFound)
}

func setDefault(c appengine.Context, r *http.Request, def string) os.Error {
	k, err := datastore.DecodeKey(def)
	if err != nil {
		return err
	}
	var config Config
	ck := datastore.NewKey(c, "$Config", "config", 0, nil)
	config.Default = k
	c.Infof("new default page: %v", k)
	if _, err := datastore.Put(c, ck, &config); err != nil {
		c.Errorf("wrong config key?")
		return err
	}
	return nil
}

func newPage(c appengine.Context, r *http.Request) os.Error {
	name := template.URLQueryEscaper(r.FormValue("name"))
	if len(name) == 0 {
		return os.NewError("field 'Name' must not be empty")
	}
	base := r.FormValue("base")
	if len(base) == 0 {
		return os.NewError("field 'Base template' must not be empty")
	}
	file := r.FormValue("file")
	if len(file) == 0 {
		return os.NewError("field  'Template' must not be empty")
	}
	key := datastore.NewKey(c, "$Pages", name, 0, nil)
	c.Infof("new key: %#v", key)
	p := Page{
		Name:     name,
		Base:     base,
		Template: file,
	}
	c.Infof("new page %#v", p)
	if _, err := datastore.Put(c, key, &p); err != nil {
		return err
	}
	return nil
}

func editPage(c appengine.Context, r *http.Request, k *datastore.Key) os.Error {
	var p Page
	if err := datastore.Get(c, k, &p); err != nil {
		return err
	}
	p.Base = r.FormValue("base")
	p.Template = r.FormValue("file")
	c.Infof("changed page %#v", p)
	if _, err := datastore.Put(c, k, &p); err != nil {
		return err
	}
	return nil
}

func getTemplate(w http.ResponseWriter, c appengine.Context, r *http.Request, k *datastore.Key) os.Error {
	if k.Kind() != "$Pages" {
		return os.NewError("it is not a page")
	}
	var p Page
	if err := datastore.Get(c, k, &p); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "multipart/form-data; charset=utf-8")
	_, err := io.WriteString(w, p.Template)
	return err
}

func exportPages(c appengine.Context, w io.Writer) os.Error {
	q := datastore.NewQuery("$Pages")
	var p []Page
	if _, err := q.GetAll(c, &p); err != nil {
		return err
	}
	b := bytes.NewBuffer(nil)
	z := zip.NewWriter(b)
	j, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}
	if zw, err := z.Create("pages"); err != nil {
		return err
	} else if _, err := zw.Write(j); err != nil {
		return err
	}
	z.Close()
	if _, err := w.Write(b.Bytes()); err != nil {
		return err
	}
	return nil
}

func importPages(c appengine.Context, file io.ReaderAt, size int64) os.Error {
	r, err := zip.NewReader(file, size)
	if err != nil {
		return err
	}
	for _, v := range r.File {
		d := make([]byte, v.UncompressedSize)
		if rc, err := v.Open(); err != nil {
			return err
		} else if _, err := io.ReadFull(rc, d); err != nil {
			c.Errorf("reading of file has failed: %q", err)
			//return err
		} else {
			rc.Close()
		}
		var p []Page
		if err := json.Unmarshal(d, &p); err != nil {
			c.Errorf("json can't unmarshal: %q", err)
			return err
		}
		for _, v := range p {
			v.Name = strings.ToLower(v.Name)
			key := datastore.NewKey(c, "$Pages", v.Name, 0, nil)
			if _, err := datastore.Put(c, key, &v); err != nil {
				return err
			}
		}
	}

	return nil
}
