package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
)

func getLogin(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.HTML(http.StatusOK, "login.tmpl", gin.H{
		"Message": "ECサイトで爆買いしよう！！！！",
	})
}

func postLogin(c *gin.Context) {
	email := c.PostForm("email")
	pass := c.PostForm("password")

	session := sessions.Default(c)
	user, result := authenticate(email, pass)
	if result {
		// 認証成功
		session.Set("uid", user.ID)
		session.Save()

		user.UpdateLastLogin()

		c.Redirect(http.StatusSeeOther, "/")
	} else {
		// 認証失敗
		c.HTML(http.StatusOK, "login.tmpl", gin.H{
			"Message": "ログインに失敗しました",
		})
	}
}

func getLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	c.Redirect(http.StatusFound, "/login")
}

func getIndex(c *gin.Context) {
	var buf bytes.Buffer
	buf.Grow(0x10000)

	io.WriteString(&buf, `<!DOCTYPE html><html><head><meta http-equiv="Content-Type" content="text/html" charset="utf-8"><link rel="stylesheet" href="/css/bootstrap.min.css"><title>すごいECサイト</title></head><body><nav class="navbar navbar-inverse navbar-fixed-top"><div class="container"><div class="navbar-header"><a class="navbar-brand" href="/">すごいECサイトで爆買いしよう!</a></div><div class="header clearfix">`)

	cUser := currentUser(c, sessions.Default(c))
	if cUser.ID > 0 {
		io.WriteString(&buf, `<nav><ul class="nav nav-pills pull-right"><li role="presentation"><a href="/users/`)
		io.WriteString(&buf, strconv.Itoa(cUser.ID))
		io.WriteString(&buf, `">`)
		io.WriteString(&buf, cUser.Name)
		io.WriteString(&buf, `さんの購入履歴</a></li><li role="presentation"><a href="/logout">Logout</a></li></ul></nav>`)
	} else {
		io.WriteString(&buf, `<nav><ul class="nav nav-pills pull-right"><li role="presentation"><a href="/login">Login</a></li></ul></nav>`)
	}

	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		page = 0
	}

	io.WriteString(&buf, `</div></nav><div class="jumbotron"><div class="container"><h1>今日は大安売りの日です！</h1></div></div><div class="container"><div class="row">`)
	if v, ok := pageCache.Load(page); ok {
		buf.Write(v.([]byte))
	} else {
		var pbuf bytes.Buffer

		products := getProductsWithCommentsAt(c, page)
		// shorten description and comment
		for _, p := range products {
			pid := strconv.Itoa(p.ID)
			io.WriteString(&pbuf, `<div class="col-md-4"><div class="panel panel-default"><div class="panel-heading"><a href="/products/`)
			io.WriteString(&pbuf, pid)
			io.WriteString(&pbuf, `">`)
			io.WriteString(&pbuf, p.Name)
			io.WriteString(&pbuf, `</a></div><div class="panel-body"><a href="/products/`)
			io.WriteString(&pbuf, pid)
			io.WriteString(&pbuf, `"><img src="`)
			io.WriteString(&pbuf, p.ImagePath)
			io.WriteString(&pbuf, `" class="img-responsive" /></a><h4>価格</h4><p>`)
			io.WriteString(&pbuf, strconv.Itoa(p.Price))
			io.WriteString(&pbuf, `円</p><h4>商品説明</h4><p>`)
			if p.ShortDescription != "" {
				io.WriteString(&pbuf, p.ShortDescription)
			} else {
				io.WriteString(&pbuf, p.Description)
			}
			io.WriteString(&pbuf, `</p><h4>`)
			io.WriteString(&pbuf, strconv.Itoa(p.CommentCount))
			io.WriteString(&pbuf, `件のレビュー</h4><ul>`)

			for _, c := range p.Comments {
				io.WriteString(&pbuf, `<li>`)
				if c.ShortContent != "" {
					io.WriteString(&pbuf, c.ShortContent)
				} else {
					io.WriteString(&pbuf, c.Content)
				}
				io.WriteString(&pbuf, ` by `)
				io.WriteString(&pbuf, c.Writer)
				io.WriteString(&pbuf, `</li>`)
			}
			io.WriteString(&pbuf, `</ul></div>`)

			if cUser.ID > 0 {
				io.WriteString(&pbuf, `<div class="panel-footer"><form method="POST" action="/products/buy/`)
				io.WriteString(&pbuf, pid)
				io.WriteString(&pbuf, `"><fieldset><input class="btn btn-success btn-block" type="submit" name="buy" value="購入" /></fieldset></form></div>`)
			}

			io.WriteString(&pbuf, `</div></div>`)
		}
		b := pbuf.Bytes()
		buf.Write(b)
		pageCache.Store(page, b)
	}
	io.WriteString(&buf, `</div></div></body></html>`)

	c.DataFromReader(http.StatusOK, int64(buf.Len()), "text/html", &buf, nil)
}

func getUserHistory(c *gin.Context) {
	cUser := currentUser(c, sessions.Default(c))
	uid, _ := strconv.Atoi(c.Param("userId"))

	var hhc map[int]*bytes.Buffer
	fmt.Println("/users/:userId", uid, cUser.ID)
	if v, ok := historyHTMLCache.Load(uid); ok {
		hhc = v.(map[int]*bytes.Buffer)
		if buf, ok := hhc[cUser.ID]; ok {
			fmt.Println("cache hit", buf.Len())
			c.DataFromReader(http.StatusOK, int64(buf.Len()), "text/html", buf, nil)
			return
		}
	} else {
		hhc = make(map[int]*bytes.Buffer)
	}
	fmt.Println("cache miss")

	user := getUser(c, uid)

	products := user.BuyingHistory(c)

	var totalPay int
	for _, p := range products {
		totalPay += p.Price
	}

	var buf bytes.Buffer
	buf.Grow(0x10000)

	io.WriteString(&buf, `<!DOCTYPE html><html><head><meta http-equiv="Content-Type" content="text/html" charset="utf-8"><link rel="stylesheet" href="/css/bootstrap.min.css"><title>すごいECサイト</title></head><body><nav class="navbar navbar-inverse navbar-fixed-top"><div class="container"><div class="navbar-header"><a class="navbar-brand" href="/">すごいECサイトで爆買いしよう!</a></div><div class="header clearfix">`)
	if cUser.ID > 0 {
		io.WriteString(&buf, `<nav><ul class="nav nav-pills pull-right"><li role="presentation"><a href="/users/`+strconv.Itoa(cUser.ID)+`">`+cUser.Name+`さんの購入履歴</a></li><li role="presentation"><a href="/logout">Logout</a></li></ul></nav>`)
	} else {
		io.WriteString(&buf, `<nav><ul class="nav nav-pills pull-right"><li role="presentation"><a href="/login">Login</a></li></ul></nav>`)
	}
	io.WriteString(&buf, `</div></nav><div class="jumbotron"><div class="container"><h2>`)
	io.WriteString(&buf, user.Name)
	io.WriteString(&buf, ` さんの購入履歴</h2><h4>合計金額: `)
	io.WriteString(&buf, strconv.Itoa(totalPay))
	io.WriteString(&buf, `円</h4></div></div><div class="container"><div class="row">`)

	// shorten description
	var productsHTML string
	for i, p := range products {
		if i >= 30 {
			break
		}
		io.WriteString(&buf, `<div class="col-md-4"><div class="panel panel-default"><div class="panel-heading"><a href="/products/`)
		io.WriteString(&buf, strconv.Itoa(p.ID))
		io.WriteString(&buf, `">`)
		io.WriteString(&buf, p.Name)
		io.WriteString(&buf, `</a></div><div class="panel-body"><a href="/products/`)
		io.WriteString(&buf, strconv.Itoa(p.ID))
		io.WriteString(&buf, `"><img src="`)
		io.WriteString(&buf, p.ImagePath)
		io.WriteString(&buf, `" class="img-responsive" /></a><h4>価格</h4><p>`)
		io.WriteString(&buf, strconv.Itoa(p.Price))
		io.WriteString(&buf, `円</p><h4>商品説明</h4><p>`)
		if p.ShortDescription != "" {
			io.WriteString(&buf, p.ShortDescription)
		} else {
			io.WriteString(&buf, p.Description)
		}
		io.WriteString(&buf, `</p><h4>購入日時</h4><p>`)
		io.WriteString(&buf, p.CreatedAt)
		io.WriteString(&buf, `</p></div>`)
		if user.ID == cUser.ID {
			io.WriteString(&buf, `<div class="panel-footer"><form method="POST" action="/comments/`)
			io.WriteString(&buf, strconv.Itoa(p.ID))
			io.WriteString(&buf, `"><fieldset><div class="form-group"><input class="form-control" placeholder="Comment Here" name="content" value=""></div><input class="btn btn-success btn-block" type="submit" name="send_comment" value="コメントを送信" /></fieldset></form></div>`)
		}
		io.WriteString(&buf, `</div></div>`)
	}
	io.WriteString(&buf, productsHTML)
	io.WriteString(&buf, `</div></div></body></html>`)

	c.DataFromReader(http.StatusOK, int64(buf.Len()), "text/html", &buf, nil)

	{
		hhc[cUser.ID] = &buf
		historyHTMLCache.Store(uid, hhc)
	}
}

func getProductPage(c *gin.Context) {
	pid, _ := strconv.Atoi(c.Param("productId"))
	product := getProduct(c, pid)
	comments := getComments(pid)

	cUser := currentUser(c, sessions.Default(c))
	bought := product.isBought(cUser.ID)

	c.HTML(http.StatusOK, "product.tmpl", gin.H{
		"CurrentUser":   cUser,
		"Product":       product,
		"Comments":      comments,
		"AlreadyBought": bought,
	})
}

func buyProduct(c *gin.Context) {
	// need authenticated
	if notAuthenticated(sessions.Default(c)) {
		c.HTML(http.StatusForbidden, "login.tmpl", gin.H{
			"Message": "先にログインをしてください",
		})
	} else {
		// buy product
		cUser := currentUser(c, sessions.Default(c))
		pid, err := strconv.Atoi(c.Param("productId"))
		if err != nil {
			panic(err.Error())
		}
		cUser.BuyProduct(c, pid)

		// redirect to user page
		c.Redirect(http.StatusFound, "/users/"+strconv.Itoa(cUser.ID))
	}
}

func comment(c *gin.Context) {
	// need authenticated
	if notAuthenticated(sessions.Default(c)) {
		c.HTML(http.StatusForbidden, "login.tmpl", gin.H{
			"Message": "先にログインをしてください",
		})
	} else {
		// create comment
		cUser := currentUser(c, sessions.Default(c))
		cUser.CreateComment(c.Param("productId"), c.PostForm("content"))

		// redirect to user page
		c.Redirect(http.StatusFound, "/users/"+strconv.Itoa(cUser.ID))
	}
}
