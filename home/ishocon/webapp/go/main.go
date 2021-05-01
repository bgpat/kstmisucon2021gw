package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/XSAM/otelsql"
	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/trace/jaeger"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var db *sql.DB

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

var tracer = otel.Tracer("webapp")

func main() {
	exporter, err := jaeger.NewRawExporter(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://localhost:14268/api/traces")))
	if err != nil {
		panic(err)
	}
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	// database setting
	user := getEnv("ISHOCON1_DB_USER", "ishocon")
	pass := getEnv("ISHOCON1_DB_PASSWORD", "ishocon")
	dbname := getEnv("ISHOCON1_DB_NAME", "ishocon1")
	driver, _ := otelsql.Register("mysql", "mysql")
	db, _ = sql.Open(driver, user+":"+pass+"@/"+dbname)
	db.SetMaxIdleConns(5)

	r := gin.Default()
	// load templates
	r.Use(static.Serve("/css", static.LocalFile("public/css", true)))
	r.Use(static.Serve("/images", static.LocalFile("public/images", true)))

	// session store
	store := sessions.NewCookieStore([]byte("mysession"))
	store.Options(sessions.Options{HttpOnly: true})
	r.Use(sessions.Sessions("showwin_happy", store))

	// GET /login
	r.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		r.SetHTMLTemplate(loginTemplate)
		c.HTML(http.StatusOK, "login", gin.H{
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
			r.SetHTMLTemplate(loginTemplate)
			c.HTML(http.StatusOK, "login", gin.H{
				"Message": "ログインに失敗しました",
			})
		}
	})

	// GET /logout
	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		r.SetHTMLTemplate(loginTemplate)
		c.Redirect(http.StatusFound, "/login")
	})

	// GET /
	r.GET("/", func(c *gin.Context) {
		ctx, span := tracer.Start(c, "GET /")
		defer span.End()

		cUser := currentUser(ctx, sessions.Default(c))

		page, err := strconv.Atoi(c.Query("page"))
		if err != nil {
			page = 0
		}
		products := getProductsWithCommentsAt(ctx, page)
		// shorten description and comment
		var sProducts []ProductWithComments
		_, sSpan := tracer.Start(ctx, "sProducts")
		for _, p := range products {
			if p.ShortDescription != "" {
				p.Description = p.ShortDescription
			}

			var newCW []CommentWriter
			for _, c := range p.Comments {
				if utf8.RuneCountInString(c.Content) > 25 {
					c.Content = string([]rune(c.Content)[:25]) + "…"
				}
				newCW = append(newCW, c)
			}
			p.Comments = newCW
			sProducts = append(sProducts, p)
		}
		sSpan.End()

		_, renderSpan := tracer.Start(ctx, "render")
		defer renderSpan.End()
		r.SetHTMLTemplate(indexTemplate)
		c.HTML(http.StatusOK, "base", gin.H{
			"CurrentUser": cUser,
			"Products":    sProducts,
		})
	})

	// GET /users/:userId
	r.GET("/users/:userId", func(c *gin.Context) {
		ctx, span := tracer.Start(c, "GET /users/:userId")
		defer span.End()

		cUser := currentUser(ctx, sessions.Default(c))

		uid, _ := strconv.Atoi(c.Param("userId"))
		user := getUser(ctx, uid)

		products := user.BuyingHistory(ctx)

		var totalPay int
		for _, p := range products {
			totalPay += p.Price
		}

		// shorten description
		sdProducts := make([]Product, 0, len(products))
		for _, p := range products {
			if p.ShortDescription != "" {
				p.Description = p.ShortDescription
			}
			sdProducts = append(sdProducts, p)
		}

		_, renderSpan := tracer.Start(ctx, "render")
		defer renderSpan.End()
		r.SetHTMLTemplate(mypageTemplate)
		c.HTML(http.StatusOK, "base", gin.H{
			"CurrentUser": cUser,
			"User":        user,
			"Products":    sdProducts,
			"TotalPay":    totalPay,
		})
	})

	// GET /products/:productId
	r.GET("/products/:productId", func(c *gin.Context) {
		ctx, span := tracer.Start(c, "GET /products/:productId")
		defer span.End()

		pid, _ := strconv.Atoi(c.Param("productId"))
		product := getProduct(pid)
		comments := getComments(pid)

		cUser := currentUser(ctx, sessions.Default(c))
		bought := product.isBought(cUser.ID)

		r.SetHTMLTemplate(productTemplate)
		c.HTML(http.StatusOK, "base", gin.H{
			"CurrentUser":   cUser,
			"Product":       product,
			"Comments":      comments,
			"AlreadyBought": bought,
		})
	})

	// POST /products/buy/:productId
	r.POST("/products/buy/:productId", func(c *gin.Context) {
		ctx, span := tracer.Start(c, "POST /products/buy/:productId")
		defer span.End()

		// need authenticated
		if notAuthenticated(sessions.Default(c)) {
			r.SetHTMLTemplate(loginTemplate)
			c.HTML(http.StatusForbidden, "login", gin.H{
				"Message": "先にログインをしてください",
			})
		} else {
			// buy product
			cUser := currentUser(ctx, sessions.Default(c))
			pid, err := strconv.Atoi(c.Param("productId"))
			if err != nil {
				panic(err.Error())
			}
			cUser.BuyProduct(pid)

			// redirect to user page
			r.SetHTMLTemplate(mypageTemplate)
			c.Redirect(http.StatusFound, "/users/"+strconv.Itoa(cUser.ID))
		}
	})

	// POST /comments/:productId
	r.POST("/comments/:productId", func(c *gin.Context) {
		ctx, span := tracer.Start(c, "POST /comments/:productId")
		defer span.End()

		// need authenticated
		if notAuthenticated(sessions.Default(c)) {
			r.SetHTMLTemplate(loginTemplate)
			c.HTML(http.StatusForbidden, "login", gin.H{
				"Message": "先にログインをしてください",
			})
		} else {
			// create comment
			cUser := currentUser(ctx, sessions.Default(c))
			cUser.CreateComment(c.Param("productId"), c.PostForm("content"))

			// redirect to user page
			r.SetHTMLTemplate(mypageTemplate)
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
			productCache = sync.Map{}
			rows, err := db.Query("SELECT * FROM products")
			if err != nil {
				c.String(http.StatusInternalServerError, err.Error())
				return
			}
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
				productCache.Store(p.ID, p)
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
				var h userHistory
				err := rows.Scan(&uid, &h.ProductID, &h.CreatedAt)
				if err != nil {
					c.String(http.StatusInternalServerError, err.Error())
					return
				}
				var uh []userHistory
				if v, ok := historyCache.Load(uid); ok {
					uh = v.([]userHistory)
				}
				uh = append(uh, h)
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
				comments = append(comments, c)
				commentCache.Store(c.ProductID, comments)
			}
		}

		c.String(http.StatusOK, "Finish")
	})

	r.RunUnix("/tmp/webapp.sock")
}
