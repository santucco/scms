// Copyright (c) 2012 Alexander Sychev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scms

import (
	"fmt"
	"net/http"
	"html/template"
	"strconv"
	"time"
	"appengine"
	"appengine/datastore"
	"appengine/user"
)

var groupSet = template.Must(template.Must(template.New("groupSet").Funcs(funcMap).Parse(
	`
{{define "recursion"}} 
	{{range .}}
		<fieldset>
		<form action="/editor/group?gid={{.GetValue "gid"}}&id={{.Key.Encode}}" method="post">
			<fieldset>
				<legend>Record ID: {{.Key.Encode}}</legend>
					<fieldset>
						<legend>New field</legend>
						<select name="name">
							<option value="NewName" selected>New field name:
							{{$cursor := .Get .Key.Kind "" "" 0 0}}
							{{range $cursor.Fields }}
								<option value={{.}}>{{.}}
							{{end}}
						</select>
						<input type="text" name="newname" value="">
						<label>Type: 
							<select name="type">
								<option value=string selected>string
								<option value=bool>bool
								<option value=integer>integer
								<option value=float>float
								<option value=time>time
								<option value=key>key
							</select>
						</label>
						<br>
						<label>Value:<br>
							<textarea name="value" rows=10 cols=100></textarea>
						</label>
						<br>
					</fieldset>	
				{{range $k, $v := .Data}}
					<fieldset>
						<legend>{{$k}}</legend>
						<label>Type: 
							<select name="type_{{$k}}">
								<option value=string {{if Type $v  "string"}}selected{{end}}>string
								<option value=bool {{if Type $v  "bool"}}selected{{end}}>bool
								<option value=integer {{if Type $v "integer"}}selected{{end}}>integer
								<option value=float {{if Type $v "float"}}selected{{end}}>float
								<option value=time {{if Type $v "time"}}selected{{end}}>time
								<option value=key {{if Type $v "key"}}selected{{end}}>key
							</select>
						</label>
						{{if Type $v "string"}}
							<br>
							<label>Value:<br>
							<textarea name="value_{{$k}}" rows=10 cols=100>{{$v}}</textarea>
						{{else}}
							<label>Value:
							{{if Type $v "time"}}
								<input type="text" name="value_{{$k}}" value={{$v}} size=100>
							{{else}}	
								<input type="text" name="value_{{$k}}" value={{$v}} size=100>
							{{end}}
						{{end}}
						</label><br>
					</fieldset>
				{{end}}
				<br>
				<input type="submit" value="Submit">
				<input type="reset" value="Reset">
				<input type="button" value="Delete">
			</fieldset>
		</form>
		{{$cursor := .Get .Key.Kind "" .Key.Encode 0 0}}
		{{if $cursor}}
			{{template "recursion" $cursor }}
		{{end}}
		<form action="/editor/group?gid={{.GetValue "gid"}}&pid={{.Key.Encode}}" method="post">
			<fieldset>
				<legend>New child record</legend>
				<select name="name">
					<option value="NewName" selected>New field name:
					{{$cursor := .Get .Key.Kind "" "" 0 0}}
					{{range $cursor.Fields }}
						<option value={{.}}>{{.}}
					{{end}}
				</select>
				<input type="text" name="newname" value="">
				<label>Type: 
					<select name="type">
						<option value=string selected>string
						<option value=bool>bool
						<option value=integer>integer
						<option value=float>float
						<option value=time>time
						<option value=key>key
					</select>
				</label>
				<br>
				<label>Value:<br>
					<textarea name="value" rows=10 cols=100></textarea>
				</label>
				<br>
				<input type="submit" value="Submit">
			</fieldset>	
		</form>
	</fieldset>	
	{{end}}
{{end}}
`)).New("group").Parse(
	`
<html>
<body>
<a href="/">Main</a><br>
<a href="/editor">Editor</a><br>
<a href="/editor/groups">Groups</a><br>
<a href="/logout">Logout</a><br>
{{$gid := .GetValue "gid"}}
{{$group := .GetByKey $gid}}
{{if $group}}
{{$name := $group.Data.Name}}
{{if $name}}
{{$cursor := .Get $name "" "" 0 0}}
<fieldset>
	<legend>Group "{{$group.Data.Name}}"</legend>
	<form action="/editor/group?gid={{.GetValue "gid"}}" method="post">
		<fieldset>
			<legend>New record</legend>
			<select name="name">
				<option value="NewName" selected>New field name:
				{{range $cursor.Fields }}
					<option value={{.}}>{{.}}
				{{end}}
			</select>
			<input type="text" name="newname" value="">
			<label>Type: 
				<select name="type">
					<option value=string selected>string
					<option value=bool>bool
					<option value=integer>integer
					<option value=float>float
					<option value=time>time
					<option value=key>key
				</select>
			</label>
			<br>
			<label>Value:<br>
				<textarea name="value" rows=10 cols=100></textarea>
			</label>
			<br>
			<input type="submit" value="Submit">
		</fieldset>
	</form>
{{template "recursion" $cursor}}
</fieldset>
{{end}}
{{end}}
</body>
</html>
`))

func groupHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if u := user.Current(c); u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	var key *datastore.Key
	parent := true
	gid := r.URL.Query().Get("gid")
	if len(gid) == 0 {
		error404(w, r)
		return
	}
	id := r.URL.Query().Get("pid")
	if len(id) == 0 {
		id = r.URL.Query().Get("id")
		parent = len(id) == 0

	}
	if len(id) != 0 {
		if k, err := datastore.DecodeKey(id); err != nil {
			errorX(c, w, err)
			return
		} else {
			key = k
		}
	}
	if r.Method == "GET" {
		var data Context
		data.ctx = c
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := groupSet.Execute(w, &data); err != nil {
			errorX(c, w, err)
		}
		return
	} else if r.Method != "POST" {
		error404(w, r)
		return
	}
	c.Infof("groupHandler: gid: %q, id: %q, parent: %q", gid, id, parent)
	if parent {
		var group string
		if key == nil {
			k, err := datastore.DecodeKey(gid)
			if err != nil {
				errorX(c, w, err)
				return
			}
			var g Group
			if err := datastore.Get(c, k, &g); err != nil {
				errorX(c, w, err)
			}
			group = g.Name
		} else {
			group = key.Kind()
		}
		if err := newRecord(c, r, group, key); err != nil {
			errorX(c, w, err)
			return
		}
	} else {
		if err := editRecord(c, r, key); err != nil {
			errorX(c, w, err)
			return
		}
	}
	r.URL.RawQuery = ""
	r.URL.Query().Add("gid", gid)
	c.Infof("RawQuery: %q", r.URL.RawQuery)
	http.Redirect(w, r, r.URL.RawQuery, http.StatusFound)
}

func newRecord(c appengine.Context, r *http.Request, g string, k *datastore.Key) error {
	name := template.HTMLEscapeString(r.FormValue("name"))
	c.Infof("newRecord: %v, %q", k, name)
	if name == "NewName" {
		name = template.HTMLEscapeString(r.FormValue("newname"))
	}
	c.Infof("newRecord: %v, %q", k, name)
	if len(name) == 0 {
		return &scmsError{"field 'Name' must not be empty"}
	}
	val := template.HTMLEscapeString(r.FormValue("value"))
	var v interface{}
	var err error
	switch r.FormValue("type") {
	case "string":
		v = val
	case "bool":
		v, err = strconv.ParseBool(val)
	case "integer":
		v, err = strconv.ParseInt(val,10,64)
	case "float":
		v, err = strconv.ParseFloat(val,64)
	case "time":
		v = time.Now()
	case "key":
		v, err = datastore.DecodeKey(val)
	default:
		err = fmt.Errorf("invalid field type %q for value %q", r.FormValue("type"), val)
	}
	c.Infof("type of field:%q, val:%q, v:%q", r.FormValue("type"), val, v)
	if err != nil {
		return err
	}
	var e entity
	e.data = Values{name: v}
	c.Infof("new Value:%v", e)
	nk := datastore.NewIncompleteKey(c, g, k)
	c.Infof("new key:%v", nk)
	if _, err := datastore.Put(c, nk, &e); err != nil {
		c.Infof("here?")
		return err
	}
	return nil
}

func editRecord(c appengine.Context, r *http.Request, k *datastore.Key) error {
	name := template.HTMLEscapeString(r.FormValue("name"))
	c.Infof("editRecord: %v, %q", k, name)
	var v interface{}
	var err error
	val := template.HTMLEscapeString(r.FormValue("value"))
	if name == "NewName" {
		name = template.HTMLEscapeString(r.FormValue("newname"))
		if len(name) != 0 {
			switch r.FormValue("type") {
			case "string":
				v = val
			case "bool":
				v, err = strconv.ParseBool(val)
			case "integer":
				v, err = strconv.ParseInt(val,10,64)
			case "float":
				v, err = strconv.ParseFloat(val,64)
			case "time":
				v = time.Now()
			case "key":
				v, err = datastore.DecodeKey(val)
			default:
				err = &scmsError{"invalid field type"}
			}
			if err != nil {
				return err
			}
			c.Infof("type of field: %q, val:%q, v:%q", r.FormValue("type"), val, v)
		}
	} else {
		var ctx = Context{ctx: c}
		cursor, err := ctx.Get(k.Kind(), "", k.Parent(), 0, 0)
		if err != nil {
			return err
		}
		for _, d := range cursor {
			for key, val := range d.Data {
				if key == name {
					v = val
					break
				}
			}
			if v != nil {
				break
			}
		}
		c.Infof("editRecord, existing name: %v, %q, %T", k, name, v)
		switch v.(type) {
		case string:
			v = val
		case bool:
			v, err = strconv.ParseBool(val)
		case int64:
			v, err = strconv.ParseInt(val,10,64)
		case float64:
			v, err = strconv.ParseFloat(val,64)
		case time.Time:
			v = time.Now()
		case datastore.Key:
			v, err = datastore.DecodeKey(val)
		default:
			err = &scmsError{"invalid field type"}
		}
		if err != nil {
			return err
		}
		c.Infof("type of field: %T, val:%q, v:%q", v, val, v)
	}
	var e entity
	if err := datastore.Get(c, k, &e); err != nil {
		return err
	}
	c.Infof("new Value:%v", e)
	for k, _ := range e.data {
		val := template.HTMLEscapeString(r.FormValue("value_" + k))
		switch r.FormValue("type_" + k) {
		case "string":
			e.data[k] = val
		case "bool":
			e.data[k], err = strconv.ParseBool(val)
		case "integer":
			e.data[k], err = strconv.ParseInt(val,10,64)
		case "float":
			e.data[k], err = strconv.ParseFloat(val,64)
		case "time":
			e.data[k] = time.Now()
		case "key":
			e.data[k], err = datastore.DecodeKey(val)
		default:
			c.Errorf("type_%s: %q", k, r.FormValue("type_"+k))
			err = &scmsError{"invalid field type"}
		}
		if err != nil {
			return err
		}
	}
	if len(name) != 0 {
		e.data[name] = v
	}
	if _, err := datastore.Put(c, k, &e); err != nil {
		return err
	}
	return nil
}
