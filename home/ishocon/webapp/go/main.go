package main

import (
	"database/sql"
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
	r.GET("/login", getLogin)

	// POST /login
	r.POST("/login", postLogin)

	// GET /logout
	r.GET("/logout", getLogout)

	// GET /
	r.GET("/", getIndex)

	// GET /users/:userId
	r.GET("/users/:userId", getUserHistory)

	// GET /products/:productId
	r.GET("/products/:productId", getProductPage)

	// POST /products/buy/:productId
	r.POST("/products/buy/:productId", buyProduct)

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
