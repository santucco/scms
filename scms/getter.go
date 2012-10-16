package scms

import (
	"os"
	"fmt"
	"strconv"
	"http"
	"appengine"
	"appengine/datastore"
)

type Values map[string]interface{}

type entity struct {
	data Values
}

type Context struct {
	ctx appengine.Context
}

type Value struct {
	Key      *datastore.Key `json:"$Key,omitempty"`
	Data     Values         `json:"$Data,omitempty"`
	ctx      Context        `json:"$Ctx,omitempty"`
	Children Cursor         `json:"$Children,omitempty"`
}

type Cursor []Value

type Paging struct {
	Number string
	Query  string
}

func (this Cursor) Len() int {
	return len(this)
}

func (this Cursor) Fields() []string {
	m := make(map[string]bool)
	for _, v := range this {
		for k, _ := range v.Data {
			m[k] = true, true
		}
	}
	out := make([]string, 0)
	for k, _ := range m {
		out = append(out, k)
	}
	return out
}

func (this Cursor) save(c appengine.Context, kind string, parent *datastore.Key) os.Error {
	for _, v := range this {
		//c.Infof("saving %#v", v)
		if err := v.save(c, kind, parent); err != nil {
			return err
		}
	}
	return nil
}

func (this *Context) Get(k interface{}, o interface{}, p interface{}, off interface{}, lim interface{}) (Cursor, os.Error) {
	var out Cursor
	if this.ctx == nil {
		return out, os.NewError("invalid context")
	}
	kind, ok := k.(string)
	if !ok {
		return out, fmt.Errorf("Get: unexpected type of 'kind': %T, must be string", k)
	}
	order, ok := o.(string)
	if !ok {
		return out, fmt.Errorf("Get: unexpected type 'order': %T, must be string", o)
	}
	var offset int
	switch off.(type){
		case int: offset = off.(int)
		case string: 
			var err os.Error
			offset, err = strconv.Atoi(off.(string))
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected type of 'offset': %T, must be int or string", off)
	}
	var limit int
	switch lim.(type){
		case int: limit = lim.(int)
		case string: 
			var err os.Error
			limit, err = strconv.Atoi(lim.(string))
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected type of 'limit': %T, must be int or string", lim)
	}	
	var parent *datastore.Key
	switch p.(type) {
	case string:
		if len(p.(string)) != 0 {
			var err os.Error
			parent, err = datastore.DecodeKey(p.(string))
			if err != nil {
				return out, err
			}
		}
	case *datastore.Key:
		parent = p.(*datastore.Key)
	}
	this.ctx.Infof("kind: %q; order: %q; parent %q; offset: %q; limit: %q", k, order, p, offset, limit)
	q := datastore.NewQuery(kind)
	if parent != nil {
		q.Ancestor(parent)
	}

	q.Offset(offset)
	if limit != 0 {
		q.Limit(limit)
	}
	if len(order) != 0 {
		q.Order(order)
	}
	var d []entity
	keys, err := q.GetAll(this.ctx, &d)
	if err != nil {
		return out, err
	}
	for i, v := range d {
		if !keys[i].Parent().Eq(parent) {
			continue
		}
		val := Value{
			Key:  keys[i],
			Data: v.data,
		}
		val.ctx.ctx = this.ctx
		out = append(out, val)
	}
	return out, nil
}

func (this *Context) GetByKey(k interface{}) (Value, os.Error) {
	var out Value
	if this.ctx == nil {
		return out, os.NewError("invalid context")
	}
	var key *datastore.Key
	switch k.(type) {
	case *datastore.Key:
		key = k.(*datastore.Key)
	case string:
		var err os.Error
		key, err = datastore.DecodeKey(k.(string))
		if err != nil {
			return out, err
		}
	default:
		return out, fmt.Errorf("invalid key: %q", k)
	}
	var d entity
	err := datastore.Get(this.ctx, key, &d)
	if err != nil {
		if err != datastore.ErrNoSuchEntity {
			return out, err
		}
		return out, nil
	}
	out.Key = key
	out.Data = d.data
	out.ctx.ctx = this.ctx
	return out, nil
}

func (this *Context) GetByKeyFields(k interface{}, s interface{}, i interface{}, p interface{}) (Value, os.Error) {
	var out Value
	if this.ctx == nil {
		return out, os.NewError("invalid context")
	}
	kind, ok := k.(string)
	if !ok {
		return out, fmt.Errorf("Get: unexpected type of 'kind': %T, must be string", k)
	}
	sid, ok := s.(string)
	if !ok {
		return out, fmt.Errorf("Get: unexpected type 'sid': %T, must be string", s)
	}
	var iid int64
	switch i.(type) {
		case int:	iid = int64(i.(int))
		case int64:	iid = i.(int64)
		case string: 
			var err os.Error
			iid, err = strconv.Atoi64(i.(string))
			if err != nil {
				return out, err
			}
		default:
			return out, fmt.Errorf("unexpected type of 'iid': %T, must be int64 or string", i)
	}
	var parent *datastore.Key
	switch p.(type) {
	case string:
		if len(p.(string)) != 0 {
			var err os.Error
			parent, err = datastore.DecodeKey(p.(string))
			if err != nil {
				return out, err
			}
		}
	case *datastore.Key:
		parent = p.(*datastore.Key)
	}
	key := datastore.NewKey(this.ctx, kind, sid, iid, parent)
	return this.GetByKey(key)
}

func (this *Context) GetPages(k interface{}, l interface{}) ([]Paging, os.Error) {
	if this.ctx == nil {
		return nil, os.NewError("invalid context")
	}
	kind, ok := k.(string)
	if !ok {
		return nil, fmt.Errorf("Get: unexpected type of 'kind': %T, must be string", k)
	}
	var limit int
	switch l.(type){
		case int: limit = l.(int)
		case string: 
			var err os.Error
			limit, err = strconv.Atoi(l.(string))
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected type of 'limit': %T, must ben string or int", l)
	}
	q := datastore.NewQuery(kind)
	q.Run(this.ctx)
	c, err := q.Count(this.ctx)
	this.ctx.Infof("GetPages: count for %q = %v", kind, c)
	c /= limit
	c++
	out := make([]Paging, c)
	if err != nil {
		return out, err
	}
	for i := 0; i < c; i++ {
		out[i].Number = fmt.Sprintf("%v", i+1)
		out[i].Query = fmt.Sprintf("offset=%v&limit=%v", i*limit, limit)
	}
	this.ctx.Infof("GetPages: %v", out)
	return out, nil
}

func (this *Context) GetPrev() (string, os.Error) {
	if this.ctx == nil {
		return "", os.NewError("invalid context")
	}
	r, ok := this.ctx.Request().(*http.Request)
	if !ok {
		return "", os.NewError("invalid request")
	}
	off := r.URL.Query().Get("offset")
	lim := r.URL.Query().Get("limit")
	if len(off) == 0 {
		return "", fmt.Errorf("'offset' not found")
	}
	if len(lim) == 0 {
		return "", fmt.Errorf("'limit' not found")
	}
	offset, err := strconv.Atoui(off)
	if err != nil {
		return "", err
	} else if offset == 0 {
		return "", nil
	}
	limit, err := strconv.Atoui(lim)
	if err != nil {
		return "", err
	} else if limit == 0 {
		return "", os.NewError("limit must be not zero")
	}
	if offset < limit {
		offset = 0
	} else {
		offset -= limit
	}
	return fmt.Sprintf("offset=%v&limit=%v", offset, limit), nil
}

func (this *Context) GetNext(k interface{}) (string, os.Error) {
	if this.ctx == nil {
		return "", os.NewError("invalid context")
	}
	kind, ok := k.(string)
	if !ok {
		return "", fmt.Errorf("Get: unexpected type of 'kind': %T, must be string", k)
	}
	r, ok := this.ctx.Request().(*http.Request)
	if !ok {
		return "", os.NewError("invalid request")
	}
	off := r.URL.Query().Get("offset")
	lim := r.URL.Query().Get("limit")
	if len(off) == 0 {
		return "", fmt.Errorf("'offset' not found")
	}
	if len(lim) == 0 {
		return "", fmt.Errorf("'limit' not found")
	}
	q := datastore.NewQuery(kind)
	c, err := q.Count(this.ctx)
	if err != nil {
		return "", err
	}
	offset, err := strconv.Atoui(off)
	if err != nil {
		return "", err
	}
	limit, err := strconv.Atoui(lim)
	if err != nil {
		return "", err
	} else if limit == 0 {
		return "", os.NewError("limit must be not zero")
	}
	if offset+limit > uint(c) {
		return "", nil
	}
	return fmt.Sprintf("offset=%v&limit=%v", offset+limit, limit), nil
}

func (this *Context) GetValue(v interface{}) (string, os.Error) {
	if this.ctx == nil {
		return "", os.NewError("invalid context")
	}
	value, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("Get: unexpected type of 'value': %T, must be string", v)
	}
	r, ok := this.ctx.Request().(*http.Request)
	if !ok {
		return "", os.NewError("invalid request")
	}
	return r.URL.Query().Get(value), nil
}

func (this *Context) GetTree(k interface{}) (Cursor, os.Error) {
	var out Cursor
	if this.ctx == nil {
		return out, os.NewError("invalid context")
	}
	return this.getTree(k, nil)
}

func (this *Context) getTree(k interface{}, p *datastore.Key) (Cursor, os.Error) {
	var out Cursor
	if this.ctx == nil {
		return out, os.NewError("invalid context")
	}
	var err os.Error
	out, err = this.Get(k, "", p, 0, 0)
	if err != nil {
		return out, err
	}
	this.ctx.Infof("getTree: out: %#v", out)
	for i, v := range out {
		out[i].Children, err = this.getTree(k, v.Key)
		if err != nil {
			break
		}
		this.ctx.Infof("getTree: children of %v: %#v", v, out)
	}
	return out, err
}

func (this *entity) Load(c <-chan datastore.Property) os.Error {
	this.data = make(Values)
	for p := range c {
		this.data[p.Name] = p.Value, true
	}
	return nil
}

func (this *entity) Save(c chan<- datastore.Property) os.Error {
	for k, v := range this.data {
		c <- datastore.Property{
			Name:  k,
			Value: v,
		}
	}
	close(c)
	return nil
}

func (this *Value) Get(k interface{}, order interface{}, p interface{}, offset interface{}, limit interface{}) (Cursor, os.Error) {
	return this.ctx.Get(k, order, p, offset, limit)
}

func (this *Value) GetByKey(k interface{}) (Value, os.Error) {
	return this.ctx.GetByKey(k)
}
func (this *Value) GetByKeyFields(kind interface{}, sid interface{}, iid interface{}, parent interface{}) (Value, os.Error) {
	return this.ctx.GetByKeyFields(kind, sid, iid, parent)
}

func (this *Value) GetPages(kind interface{}, limit interface{}) ([]Paging, os.Error) {
	return this.ctx.GetPages(kind, limit)
}

func (this *Value) GetPrev() (string, os.Error) {
	return this.ctx.GetPrev()
}

func (this *Value) GetNext(kind interface{}) (string, os.Error) {
	return this.ctx.GetNext(kind)
}

func (this *Value) GetValue(value interface{}) (string, os.Error) {
	return this.ctx.GetValue(value)
}

func (this *Value) GetTree(k interface{}) (Cursor, os.Error) {
	return this.ctx.GetTree(k)
}

func (this *Value) save(c appengine.Context, kind string, parent *datastore.Key) os.Error {
	if this.Key == nil {
		this.Key = datastore.NewIncompleteKey(c, kind, parent)
	}
	e := entity{data: this.Data}
	c.Infof("kind %q, parent %q", kind, parent)
	var err os.Error
	this.Key, err = datastore.Put(c, this.Key, &e)
	if err != nil {
		c.Errorf("this: %#v, entity: %v, err: %q", this, e, err)
		return err
	}
	c.Infof("new key: %q", this.Key)
	return this.Children.save(c, kind, this.Key)
}
