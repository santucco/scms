// Copyright (c) 2012 Alexander Sychev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scms

import(
	"net/http"
	"appengine"
	"appengine/user"
)

type User struct{
	Email	string
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		url, err := user.LoginURL(c, r.URL.String())
		if err != nil {
			errorX(c, w, err)
			return
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	url, err := user.LogoutURL(c, r.URL.String())
	if err != nil {
		errorX(c, w, err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}
