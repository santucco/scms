package scms

import (
	"http"
	"template"
	"os"
	"io"
	"bytes"
	"fmt"
	"archive/zip"
	"appengine"
	"appengine/datastore"
	"appengine/user"
)

type File struct {
	Name string
	Data []byte
}

var filesTemplate = template.Must(template.New("files").Parse(
	`
<html>
<body>
<a href="/">Main</a><br>
<a href="/editor">Editor</a><br>
<a href="/logout">Logout</a><br>
{{$cursor :=.Get "$Files" "Name" "" 0 0}}
<form action="/editor/files" method="post" enctype="multipart/form-data">
	<fieldset>
		<legend>New file</legend>
		<label>Name of file:<br><input type="text" name="name" value=""></label><br>
		<label>New file:<br><input type="file" name="file" value=""></label><br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
<form action="/editor/files?action=upload" method="post" enctype="multipart/form-data">
	<fieldset>
	{{if $cursor.Len}}
		<a href=/editor/files.zip>Download all files</a><br>
	{{end}}
		<label>Upload files: <input type="file" name="file" value=""></label><br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
{{range $cursor}}
<form action="/editor/files?id={{.Key.Encode}}" method="post" enctype="multipart/form-data">
	<fieldset>
		<legend>File "{{.Data.Name}}"</legend>
		<a href=/{{.Data.Name}}>Download file '{{.Data.Name}}'</a><br>
		<label>Upload new file "{{.Data.Name}}": <input type="file" name="file" value=""></label><br>
	<input type="submit" value="Submit">
	<input type="reset" value="Reset">
	<input type="button" value="Delete">
	</fieldset>
</form>
{{end}}
</body>
</html>
`))

func filesHandler(w http.ResponseWriter, r *http.Request) {
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
		var data Context
		data.ctx = c
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := filesTemplate.Execute(w, &data); err != nil {
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
			c.Errorf("can't get an archive with imported files: %q", err)
			return
		}
		if n, err := file.Seek(0, os.SEEK_END); err != nil {
			error(c, w, err)
			return
		} else if err := importFiles(c, file, n); err != nil {
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
			if err := newFile(c, r); err != nil {
				error(c, w, err)
				return
			}
		} else if err := editFile(c, r, key); err != nil {
			error(c, w, err)
			return
		}
	}
	createHandlers(c)
	http.Redirect(w, r, r.URL.Path, http.StatusFound)
}

func newFile(c appengine.Context, r *http.Request) os.Error {
	c.Infof("newFile: %#v", r)
	name := r.FormValue("name")
	if len(name) == 0 {
		return os.NewError("field 'Name' must not be empty")
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		c.Errorf("can't get template file: %q", err)
		return err
	}
	f := File{
		Name: name,
	}
	if n, err := file.Seek(0, os.SEEK_END); err != nil {
		return err
	} else if n >= 0x100000 {
		return os.NewError("file is too long")
	} else {
		f.Data = make([]byte, n)
	}
	if _, err = file.ReadAt(f.Data, 0); err != nil {
		return err
	}
	key := datastore.NewKey(c, "$Files", f.Name, 0, nil)
	c.Infof("new key: %#v", key)
	if _, err := datastore.Put(c, key, &f); err != nil {
		return err
	}
	return nil
}

func editFile(c appengine.Context, r *http.Request, k *datastore.Key) os.Error {
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil
	}
	var f File
	if err := datastore.Get(c, k, &f); err != nil {
		return err
	}
	if n, err := file.Seek(0, os.SEEK_END); err != nil {
		return err
	} else if n >= 0x100000 {
		return os.NewError("file is too long")
	} else {
		f.Data = make([]byte, n)
	}
	_, err = file.ReadAt(f.Data, 0)
	if err != nil {
		return err
	}
	if _, err := datastore.Put(c, k, &f); err != nil {
		return err
	}
	return nil
}

func exportFile(c appengine.Context, w http.ResponseWriter, fn string) os.Error {
	var f File
	c.Infof("exporting file %q", fn)
	if err := datastore.Get(c, datastore.NewKey(c, "$Files", fn, 0, nil), &f); err != nil {
		c.Errorf("file %q not found: %q", fn, err)
		return err
	}
	w.Header().Set("Content-Type", "multipart/form-data; charset=utf-8")
	if _, err := w.Write(f.Data); err != nil {
		return err
	}
	return nil
}

func exportFiles(c appengine.Context, w io.Writer) os.Error {
	q := datastore.NewQuery("$Files")
	var f []File
	if _, err := q.GetAll(c, &f); err != nil {
		return err
	}
	b := bytes.NewBuffer(nil)
	z := zip.NewWriter(b)
	for _, v := range f {
		//c.Infof("packing file: %q", v)
		if zw, err := z.Create(v.Name); err != nil {
			return err
		} else if _, err := zw.Write(v.Data); err != nil {
			return err
		}
	}
	z.Close()
	if _, err := w.Write(b.Bytes()); err != nil {
		return err
	}
	return nil
}

func importFiles(c appengine.Context, file io.ReaderAt, size int64) os.Error {
	r, err := zip.NewReader(file, size)
	if err != nil {
		return err
	}
	for _, v := range r.File {
		if v.UncompressedSize == 0 {
			continue
		}
		if v.UncompressedSize >= 0x100000 {
			return fmt.Errorf("file '%q' is too long", v.Name)
		}
		f := File{
			Name: v.Name,
			Data: make([]byte, v.UncompressedSize),
		}
		if rc, err := v.Open(); err != nil {
			return err
		} else if _, err := io.ReadFull(rc, f.Data); err != nil {
			c.Errorf("reading of file has failed: %q", err)
			return err
		} else {
			rc.Close()
		}

		key := datastore.NewKey(c, "$Files", f.Name, 0, nil)
		if _, err := datastore.Put(c, key, &f); err != nil {
			return err
		}
	}

	return nil
}
