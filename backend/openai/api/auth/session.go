package auth

import (
	"backend/config"
	"backend/modules/chatgpt/model"
	"backend/utility"
	"time"

	"github.com/cool-team-official/cool-admin-go/cool"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

var (
	errSessionStr = `
	{
		"detail": {
			"type":    "invalid_request_error",
			"param":   <nil>,
			"code":    "session_invalidated",
			"message": "Your authentication token has expired. Please try signing in again.",
		},
	}`
)

func Session(r *ghttp.Request) {
	ctx := r.GetCtx()
	usertoken := r.Session.MustGet("usertoken").String()

	carid := r.Session.MustGet("carid").String()
	carinfo, err := utility.CheckCar(ctx, carid)
	if err != nil {
		g.Log().Error(ctx, err)
		r.Response.WriteJson(g.Map{
			"code": 0,
			"msg":  "服务器错误",
		})
		return
	}

	getsessionUrl := config.CHATPROXY + "/getsession"
	getsessionVar := g.Client().PostVar(ctx, getsessionUrl, g.MapStrStr{
		"refreshCookie": carinfo.RefreshCookie,
		"authkey":       config.AUTHKEY,
	})
	sessionJson := gjson.New(getsessionVar)
	detail := sessionJson.Get("detail").String()
	if detail != "" {
		utility.CloseCar(ctx, carid)
		r.Response.Status = 401
		r.Response.WriteJson(gjson.New(errSessionStr))
		return

	}
	// sessionJson.Dump()
	email := sessionJson.Get("user.email").String()
	if email == "" {
		r.Response.WriteJson(g.Map{
			"code": 0,
			"msg":  "email is empty",
		})
		return
	}
	models := sessionJson.Get("models").Array()
	// 更新账号信息
	cool.DBM(model.NewChatgptSession()).Where("email=?", email).Update(g.Map{
		"officialSession": sessionJson.String(),
		"isPlus":          len(models) > 1,
		"status":          1,
	})
	// 更新缓存
	cool.CacheManager.Set(ctx, "session:"+carid, sessionJson.String(), 90*24*time.Hour)
	// 移除sessionJson中的refreshCookie
	sessionJson.Remove("refreshCookie")
	// 移除sessionJson中的models
	sessionJson.Remove("models")
	sessionJson.Set("user.email", "share@openai.com")
	sessionJson.Set("user.name", carid)
	sessionJson.Set("user.image", "/avatars.png")
	sessionJson.Set("user.picture", "/avatars.png")
	sessionJson.Set("user.id", "user-"+usertoken)
	sessionJson.Set("accessToken", usertoken)

	r.Response.WriteJson(sessionJson)
}
