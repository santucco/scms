// Copyright (c) 2012 Alexander Sychev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scms

import (
	"net/http"
	"html/template"
	"os"
	"io"
	"encoding/json"
	"bytes"
	"archive/zip"
	"appengine"
	"appengine/datastore"
	"appengine/user"
)

type Group struct {
	Name string
}

var groupsTemplate = template.Must(template.New("groups").Parse(
	`
<html>
<body>
<a href="/">Main</a><br>
<a href="/editor">Editor</a><br>
<a href="/logout">Logout</a><br>
<form action="/editor/groups" method="post">
	<fieldset>
		<legend>New group</legend>
		<label>Name of group:<br><input type="text" name="name" value=""></label><br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
{{$cursor :=.Get "$Groups" "Name" "" 0 0}}
<form action="/editor/groups?action=upload" method="post" enctype="multipart/form-data">
	<fieldset>
	{{if $cursor.Len}}
		<a href=/editor/groups.zip>Download all groups</a><br>
	{{end}}
		<label>Upload groups: <input type="file" name="file" value=""></label><br>
		<input type="submit" value="Submit">
	</fieldset>
</form>
{{range $cursor}}
<form action="/editor/groups?id={{.Key.Encode}}" method="post">
	<fieldset>
		<legend>Group "{{.Data.Name}}"</legend>
		<label>ID: <input type="text" name="name" value="{{.Key.Encode}}" size=60></label><br>
		<a href="/editor/group?gid={{.Key.Encode}}">Records</a><br>
		<input type="button" value="Delete">
	</fieldset>
</form>
{{end}}
</body>
</html>
`))

func groupsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if r.Method == "GET" {
		var data Context
		data.ctx = c
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := groupsTemplate.Execute(w, &data); err != nil {
			errorX(c, w, err)
		}
		return
	} else if r.Method != "POST" {
		error404(w, r)
		return
	}
	if r.FormValue("action") == "upload" {
		file, _, err := r.FormFile("file")
		if err != nil {
			errorX(c, w, err)
			c.Errorf("can't get an archive with imported groups: %q", err)
			return
		}
		if n, err := file.Seek(0, os.SEEK_END); err != nil {
			errorX(c, w, err)
			return
		} else if err := importGroups(c, file, n); err != nil {
			errorX(c, w, err)
			return
		}
	} else if id := r.URL.Query().Get("id"); len(id) == 0 {
		var err error
		err = newGroup(c, r)
		if err != nil {
			errorX(c, w, err)
			return
		}
	} else if _, err := datastore.DecodeKey(id); err != nil {
		errorX(c, w, err)
		return
	} else {
		// TODO: delete group?
	}
	createHandlers(c)
	http.Redirect(w, r, r.URL.Path, http.StatusFound)
}

func groupsEditor(w http.ResponseWriter, r *http.Request, n string) {
	c := appengine.NewContext(r)
	if r.Method == "GET" {
		q := datastore.NewQuery(n)
		var e []interface{}
		_, err := q.GetAll(c, &e)
		if err != nil {
			errorX(c, w, err)
			return
		}
		c.Infof("entities of %v: %#v", n, e)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := groupsTemplate.Execute(w, e); err != nil {
			errorX(c, w, err)
		}
		return
	} else if r.Method != "POST" {
		error404(w, r)
		return
	}
	http.Redirect(w, r, r.URL.Path, http.StatusFound)
}

func newGroup(c appengine.Context, r *http.Request) error {
	name := template.URLQueryEscaper(r.FormValue("name"))
	if len(name) == 0 {
		return &scmsError{"field 'Name' must not be empty"}
	}
	key := datastore.NewKey(c, "$Groups", name, 0, nil)
	c.Infof("new key: %#v", key)
	p := Group{
		Name: name,
	}
	c.Infof("new group %#v", p)
	if _, err := datastore.Put(c, key, &p); err != nil {
		return err
	}
	return nil
}

func exportGroups(c appengine.Context, w io.Writer) error {
	q := datastore.NewQuery("$Groups")
	var g []Group
	if _, err := q.GetAll(c, &g); err != nil {
		return err
	}
	b := bytes.NewBuffer(nil)
	z := zip.NewWriter(b)
	for _, v := range g {
		ctx := &Context{
			ctx: c,
		}
		cur, err := ctx.GetTree(v.Name)
		if err != nil {
			return err
		}
		j, err := json.MarshalIndent(cur, "", "\t")
		if err != nil {
			return err
		}
		if zw, err := z.Create(v.Name); err != nil {
			return err
		} else if _, err := zw.Write(j); err != nil {
			return err
		}
	}
	z.Close()
	if _, err := w.Write(b.Bytes()); err != nil {
		return err
	}
	return nil
}

func importGroups(c appengine.Context, file io.ReaderAt, size int64) error {
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
			return err
		} else {
			rc.Close()
		}
		var cur Cursor
		if err := json.Unmarshal(d, &cur); err != nil {
			c.Errorf("json can't unmarshal: %q", err)
			return err
		}
		g := Group{Name: v.Name}
		key := datastore.NewKey(c, "$Groups", g.Name, 0, nil)
		if _, err := datastore.Put(c, key, &g); err != nil {
			return err
		}
		cur.save(c, g.Name, nil)
	}

	return nil
}
