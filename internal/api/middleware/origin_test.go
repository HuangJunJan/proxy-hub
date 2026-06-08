package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func originRouter(allowed []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck(allowed))
	r.POST("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doReq(r *gin.Engine, method, origin string) int {
	req := httptest.NewRequest(method, "/x", nil)
	req.Host = "127.0.0.1:7777"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestOriginCheck(t *testing.T) {
	r := originRouter(nil)
	if c := doReq(r, http.MethodGet, "http://evil.com"); c != http.StatusOK {
		t.Errorf("GET 跨域应放行（只读），得 %d", c)
	}
	if c := doReq(r, http.MethodPost, ""); c != http.StatusOK {
		t.Errorf("POST 无 Origin（CLI/服务端）应放行，得 %d", c)
	}
	if c := doReq(r, http.MethodPost, "http://127.0.0.1:7777"); c != http.StatusOK {
		t.Errorf("POST 同源应放行，得 %d", c)
	}
	if c := doReq(r, http.MethodPost, "http://evil.com"); c != http.StatusForbidden {
		t.Errorf("POST 跨域应 403，得 %d", c)
	}

	r2 := originRouter([]string{"http://admin.example.com"})
	if c := doReq(r2, http.MethodPost, "http://admin.example.com"); c != http.StatusOK {
		t.Errorf("白名单 origin 应放行，得 %d", c)
	}
	if c := doReq(r2, http.MethodPost, "http://other.com"); c != http.StatusForbidden {
		t.Errorf("非白名单跨域应 403，得 %d", c)
	}
}
