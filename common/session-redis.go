package common

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/rbcervilla/redisstore/v9"
	"log"
	"net/http"
)

var DefaultKey = "github.com/gin/sessions"

type SessionData struct {
	name    string
	request *http.Request
	store   *redisstore.RedisStore
	session *sessions.Session
	written bool
	writer  http.ResponseWriter
}

func (s *SessionData) ID() string {
	return s.Session().ID
}

func (s *SessionData) Get(key interface{}) interface{} {
	return s.Session().Values[key]
}

func (s *SessionData) Set(key interface{}, val interface{}) {
	s.Session().Values[key] = val
	s.written = true
}

func (s *SessionData) Delete(key interface{}) {
	delete(s.Session().Values, key)
	s.written = true
}

func (s *SessionData) Clear() {
	for key := range s.Session().Values {
		s.Delete(key)
	}
}

func (s *SessionData) AddFlash(value interface{}, vars ...string) {
	s.Session().AddFlash(value, vars...)
	s.written = true
}

func (s *SessionData) Flashes(vars ...string) []interface{} {
	s.written = true
	return s.Session().Flashes(vars...)
}

//func (s *SessionData) Options(options Options) {
//	s.written = true
//}

func (s *SessionData) Save() error {
	if s.Written() {
		e := s.Session().Save(s.request, s.writer)
		if e == nil {
			s.written = false
		}
		return e
	}
	return nil
}

func (s *SessionData) Session() *sessions.Session {
	if s.session == nil {
		var err error
		s.session, err = s.store.Get(s.request, s.name)
		if err != nil {
			log.Printf(err.Error())
		}
	}
	return s.session
}

func (s *SessionData) Written() bool {
	return s.written
}

// GetSession shortcut to get session
func GetSession(c *gin.Context) *SessionData {
	return c.MustGet(DefaultKey).(*SessionData)
}

func SessionMiddleware(name string, store *redisstore.RedisStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		s := &SessionData{name, c.Request, store, nil, false, c.Writer}
		c.Set(DefaultKey, s)
		defer context.Clear(c.Request)
		c.Next()
	}
}
