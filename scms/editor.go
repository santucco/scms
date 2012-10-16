package scms

import (
	"os"
	"io"
	"fmt"
	"http"
	"bytes"
	"archive/zip"
	"template"
	"appengine"
	"appengine/user"
	"appengine/datastore"
)

var editorTemplate = template.Must(template.New("editor").Parse(
	`
<html>
<body>
<a href="/">Main</a><br>
<a href="/editor/files">Files</a><br>
<a href="/editor/pages">Pages</a><br>
<a href="/editor/groups">Groups</a><br>
<br>
<form action="/editor/?action=upload" method="post" enctype="multipart/form-data">
	<fieldset>
		<a href=/editor/all.zip>Download entire the site</a><br>
		<label>Upload entire the site: <input type="file" name="file" value=""></label><br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
<br>
<a href="/logout">Logout</a><br>	
</body>
</html>
`))

func editorHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "multipart/form-data; charset=utf-8")
		switch r.URL.Path {
		case "/editor/files.zip":
			exportFiles(c, w)
			return
		case "/editor/pages.zip":
			if err := exportPages(c, w); err != nil {
				error(c, w, err)
			}
			return
		case "/editor/groups.zip":
			if err := exportGroups(c, w); err != nil {
				error(c, w, err)
			}
			return
		case "/editor/all.zip":
			if err := exportAll(c, w); err != nil {
				error(c, w, err)
			}
			return
		default:
			if err := exportFile(c, w, r.URL.Path[1:]); err == nil {
				return
			}
		}

		var data Context
		data.ctx = c
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := editorTemplate.Execute(w, &data); err != nil {
			error(c, w, err)
		}
		return
	}
	if r.Method != "POST" {
		error404(w, r)
	}
	if r.FormValue("action") == "upload" {
		file, _, err := r.FormFile("file")
		if err != nil {
			error(c, w, err)
			c.Errorf("can't get an archive with imported site: %q", err)
			return
		}
		if n, err := file.Seek(0, os.SEEK_END); err != nil {
			error(c, w, err)
			return
		} else if err := importAll(c, file, n); err != nil {
			error(c, w, err)
			return
		}
	}
	http.Redirect(w, r, "/editor", http.StatusFound)
}

var funcMap = template.FuncMap{"Type": isType, "EqualString": equalString}

func equalString(i1 interface{}, i2 interface{}) (bool, os.Error) {
	s1, ok := i1.(string)
	if !ok {
		return false, fmt.Errorf("invalid type of argument %T, must be string", i1)
	}
	s2, ok := i2.(string)
	if !ok {
		return false, fmt.Errorf("invalid type of argument %T, must be string", i2)
	}
	return s1 == s2, nil
}

func isType(t interface{}, s string) (bool, os.Error) {
	switch t.(type) {
	case string:
		return s == "string", nil
	case bool:
		return s == "bool", nil
	case int64:
		return s == "integer", nil
	case float64:
		return s == "float", nil
	case datastore.Time:
		return s == "time", nil
	case datastore.Key:
		return s == "key", nil
	}
	return false, fmt.Errorf("type %T is unsupported", t)
}

func exportAll(c appengine.Context, w io.Writer) os.Error {
	b := bytes.NewBuffer(nil)
	z := zip.NewWriter(b)
	if wz, err := z.Create("files.zip"); err != nil {
		return err
	} else {
		exportFiles(c, wz)
	}
	if wz, err := z.Create("pages.zip"); err != nil {
		return err
	} else {
		exportPages(c, wz)
	}
	if wz, err := z.Create("groups.zip"); err != nil {
		return err
	} else {
		exportGroups(c, wz)
	}
	z.Close()
	if _, err := w.Write(b.Bytes()); err != nil {
		return err
	}
	return nil
}

func importAll(c appengine.Context, file io.ReaderAt, size int64) os.Error {
	r, err := zip.NewReader(file, size)
	if err != nil {
		return err
	}
	for _, v := range r.File {
		if v.UncompressedSize == 0 {
			continue
		}
		c.Infof("importing %q", v.Name)
		d := make(customReaderAt, v.UncompressedSize)
		if rc, err := v.Open(); err != nil {
			return err
		} else if _, err := io.ReadFull(rc, d); err != nil {
			c.Errorf("reading of file has failed: %q", err)
			return err
		} else {
			rc.Close()
		}
		switch v.Name {
		case "files.zip":
			if err := importFiles(c, d, int64(len(d))); err != nil {
				return err
			}
		case "pages.zip":
			if err := importPages(c, d, int64(len(d))); err != nil {
				return err
			}
		case "groups.zip":
			if err := importGroups(c, d, int64(len(d))); err != nil {
				return err
			}
		}

	}
	return nil
}

type customReaderAt []byte

func (this customReaderAt) ReadAt(p []byte, off int64) (n int, err os.Error) {
	if off > int64(len(this)) {
		return 0, os.EOF
	}
	n = copy(p, this[off:])
	if n < len(p) {
		return n, os.EOF
	}
	return n, nil
}
