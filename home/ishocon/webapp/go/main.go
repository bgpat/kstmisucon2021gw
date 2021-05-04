package main

import (
	"bytes"
	"database/sql"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	// database setting
	user := getEnv("ISHOCON1_DB_USER", "ishocon")
	pass := getEnv("ISHOCON1_DB_PASSWORD", "ishocon")
	dbname := getEnv("ISHOCON1_DB_NAME", "ishocon1")
	db, _ = sql.Open("mysql", user+":"+pass+"@/"+dbname)
	db.SetMaxIdleConns(5)

	r := gin.Default()
	pprof.Register(r)
	// load templates
	r.Use(static.Serve("/css", static.LocalFile("public/css", true)))
	r.Use(static.Serve("/images", static.LocalFile("public/images", true)))
	r.LoadHTMLGlob("templates/*")

	// session store
	store := sessions.NewCookieStore([]byte("mysession"))
	store.Options(sessions.Options{HttpOnly: true})
	r.Use(sessions.Sessions("showwin_happy", store))

	// GET /login
	r.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		c.HTML(http.StatusOK, "login.tmpl", gin.H{
			"Message": "ECサイトで爆買いしよう！！！！",
		})
	})

	// POST /login
	r.POST("/login", func(c *gin.Context) {
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
	})

	// GET /logout
	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		c.Redirect(http.StatusFound, "/login")
	})

	// GET /
	r.GET("/", func(c *gin.Context) {
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
	})

	// GET /users/:userId
	r.GET("/users/:userId", func(c *gin.Context) {
		cUser := currentUser(c, sessions.Default(c))

		uid, _ := strconv.Atoi(c.Param("userId"))
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
	})

	// GET /products/:productId
	r.GET("/products/:productId", func(c *gin.Context) {
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
	})

	// POST /products/buy/:productId
	r.POST("/products/buy/:productId", func(c *gin.Context) {
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
	})

	// POST /comments/:productId
	r.POST("/comments/:productId", func(c *gin.Context) {
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
	})

	// GET /initialize
	r.GET("/initialize", func(c *gin.Context) {
		db.Exec("DELETE FROM users WHERE id > 5000")
		db.Exec("DELETE FROM products WHERE id > 10000")
		db.Exec("DELETE FROM comments WHERE id > 200000")
		db.Exec("DELETE FROM histories WHERE id > 500000")

		{
			userCache = sync.Map{}
			rows, err := db.Query("SELECT * FROM users")
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			for rows.Next() {
				var u User
				err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.LastLogin)
				if err != nil {
					c.String(http.StatusInternalServerError, err.Error())
					return
				}
				userCache.Store(u.ID, u)
			}
		}

		{
			productCache = make(map[int]*Product)
			pages = make(map[int]int)
			rows, err := db.Query("SELECT * FROM products ORDER BY id DESC")
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			var i int
			for rows.Next() {
				var p Product
				err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.ImagePath, &p.Price, &p.CreatedAt)
				if err != nil {
					c.String(http.StatusInternalServerError, err.Error())
					return
				}
				if utf8.RuneCountInString(p.Description) > 70 {
					p.ShortDescription = string([]rune(p.Description)[:70]) + "…"
				}
				productCache[p.ID] = &p

				pages[p.ID] = i / 50
				i++
			}
		}

		{
			historyCache = sync.Map{}
			rows, err := db.Query("SELECT user_id, product_id, created_at FROM histories ORDER BY id DESC")
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			for rows.Next() {
				var uid int
				var h Product
				err := rows.Scan(&uid, &h.ID, &h.CreatedAt)
				if err != nil {
					c.String(http.StatusInternalServerError, err.Error())
					return
				}
				p := getProduct(c, h.ID)
				p.CreatedAt = h.CreatedAt
				var uh []Product
				if v, ok := historyCache.Load(uid); ok {
					uh = v.([]Product)
				}
				uh = append(uh, p)
				historyCache.Store(uid, uh)
			}
		}

		{
			_c := c
			commentCache = sync.Map{}
			rows, err := db.Query("SELECT * FROM comments ORDER BY created_at DESC")
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
			for rows.Next() {
				var c Comment
				err := rows.Scan(&c.ID, &c.ProductID, &c.UserID, &c.Content, &c.CreatedAt)
				if err != nil {
					_c.String(http.StatusInternalServerError, err.Error())
					return
				}
				var comments []Comment
				if v, ok := commentCache.Load(c.ProductID); ok {
					comments = v.([]Comment)
				}
				if utf8.RuneCountInString(c.Content) > 70 {
					c.ShortContent = string([]rune(c.Content)[:70]) + "…"
				}
				comments = append(comments, c)
				commentCache.Store(c.ProductID, comments)
			}
		}

		c.String(http.StatusOK, "Finish")
	})

	r.Run(":80")
}
