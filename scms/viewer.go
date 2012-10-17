// Copyright (c) 2012 Alexander Sychev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scms

import (
	"net/http"
	"html/template"
	ttpl "text/template"
	"bytes"
	"appengine"
	"appengine/datastore"
)

type scmsError struct {
	err string
}

var created = false
var paths = make(map[string]bool)

func init() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/editor/", editorHandler)
	http.HandleFunc("/editor/pages", pagesHandler)
	http.HandleFunc("/editor/groups", groupsHandler)
	http.HandleFunc("/editor/group", groupHandler)
	http.HandleFunc("/editor/files", filesHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
}

func errorX(c appengine.Context, w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	//	io.WriteString(w, "Oops! Internal Server Error\n")
	//	io.WriteString(w, err.String())
	c.Errorf("%v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func error404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.NotFound(w, r)
	//io.WriteString(w, "Oops! Not Found.")
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		error404(w, r)
		return
	}
	c := appengine.NewContext(r)
	c.Infof("URL: %#v", r.URL)
	if r.URL.Path == "/" {
		var config Config
		datastore.Get(c, datastore.NewKey(c, "$Config", "config", 0, nil), &config)
		if config.Default != nil {
			c.Infof("%q is default page", config.Default.StringID())
			http.Redirect(w, r, "/"+config.Default.StringID(), http.StatusFound)
			return
		}
		errorX(c, w, &scmsError{"no default page is specified"})
		return
	} else if err := exportFile(c, w, r.URL.Path[1:]); err == nil {
		return
	}
	if !created {
		if err := createHandlers(c); err != nil {
			http.Redirect(w, r, "/editor", http.StatusFound)
			return
		}
		created = true
	}
	if _, ok := paths[r.URL.Path]; ok {
		http.Redirect(w, r, r.URL.RawQuery, http.StatusFound)
	} else {
		http.NotFound(w, r)
	}
}

func createHandlers(c appengine.Context) error {
	q := datastore.NewQuery("$Pages")
	var p []Page
	q.GetAll(c, &p)

	if len(p) == 0 {
		c.Infof("pages are not found, redirecting to the editor")
		return &scmsError{"pages not found"}
	}
	c.Infof("pages: %#v", p)
	for _, v := range p {
		c.Infof("checking page: %#v", v)
		if len(v.Name) == 0 || len(v.Template) == 0 {
			c.Infof("an empty page was found, continue")
			continue
		}
		h, err := createHandler(c, v)
		if err != nil {
			c.Errorf("createHandler returns error: %q", err)
			return err
		}
		c.Infof("handling  page: %#v, handler: %#v", v.Name, h)
		http.HandleFunc("/"+v.Name, h)
		paths["/"+v.Name] = true
	}
	if len(paths) == 0 {
		return &scmsError{"pages not found"}
	}
	return nil
}

func createHandler(c appengine.Context, p Page) (func(w http.ResponseWriter, r *http.Request), error) {
	b := bytes.NewBuffer(nil)
	var base File
	if err := datastore.Get(c, datastore.NewKey(c, "$Files", p.Base, 0, nil), &base); err != nil {
		return nil, err
	}
	c.Infof("base template: %q", string(base.Data))
	var templ File
	if err := datastore.Get(c, datastore.NewKey(c, "$Files", p.Template, 0, nil), &templ); err != nil {
		return nil, err
	}
	c.Infof("template: %q", string(templ.Data))
	if tpl, err := ttpl.New(p.Base).Parse(string(base.Data)); err != nil {
		return nil, err
	} else if err := tpl.Execute(b, string(templ.Data)); err != nil {
		return nil, err
	}
	c.Infof("creating handler for page %#v", p.Name)
	c.Infof("ready template: %q", b.String())
	tpl, err := template.New(p.Name).Parse(b.String())
	if err != nil {
		return nil, err
	}
	name := "/" + p.Name
	tpl.Funcs(funcMap)
	return func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		c.Infof("request in custom handler of '%#v': %#v", name, r)
		if r.Method != "GET" {
			error404(w, r)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != name {
			error404(w, r)
			return
		}
		ctx := Context{
			ctx: c,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tpl.Execute(w, &ctx); err != nil {
			c.Errorf("%v", err)
		}
	}, nil
}

func (this scmsError) Error() string {
	return this.err
}