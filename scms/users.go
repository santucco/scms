package scms

import(
	"http"
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
			error(c, w, err)
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
		error(c, w, err)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}
