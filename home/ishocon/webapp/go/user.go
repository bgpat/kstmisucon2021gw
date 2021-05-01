package main

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/contrib/sessions"
)

var (
	userCache    sync.Map
	historyCache sync.Map
)

// User model
type User struct {
	ID        int
	Name      string
	Email     string
	Password  string
	LastLogin string
}

type userHistory struct {
	ProductID int
	CreatedAt string
}

func authenticate(email string, password string) (User, bool) {
	var u User
	err := db.QueryRow("SELECT * FROM users WHERE email = ? LIMIT 1", email).Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.LastLogin)
	if err != nil {
		return u, false
	}
	result := password == u.Password
	return u, result
}

func notAuthenticated(session sessions.Session) bool {
	uid := session.Get("uid")
	return !(uid.(int) > 0)
}

func getUser(ctx context.Context, uid int) User {
	ctx, span := tracer.Start(ctx, "getUser")
	defer span.End()

	u := User{}
	r := db.QueryRow("SELECT * FROM users WHERE id = ? LIMIT 1", uid)
	err := r.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.LastLogin)
	if err != nil {
		return u
	}

	return u
}

func currentUser(ctx context.Context, session sessions.Session) User {
	uid := session.Get("uid")
	u := User{}
	r := db.QueryRow("SELECT * FROM users WHERE id = ? LIMIT 1", uid)
	err := r.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.LastLogin)
	if err != nil {
		return u
	}

	return u
}

// BuyingHistory : products which user had bought
func (u *User) BuyingHistory(ctx context.Context) (products []Product) {
	ctx, span := tracer.Start(ctx, "BuyingHistory")
	defer span.End()

	var uh []userHistory
	if v, ok := historyCache.Load(u.ID); ok {
		uh = v.([]userHistory)
	}
	for _, h := range uh {
		p := Product{}
		p = getProduct(h.ProductID)
		fmt := "2006-01-02 15:04:05"
		tmp, _ := time.Parse(fmt, h.CreatedAt)
		p.CreatedAt = (tmp.Add(9 * time.Hour)).Format(fmt)

		products = append(products, p)
	}

	return
}

// BuyProduct : buy product
func (u *User) BuyProduct(pid int) {
	now := time.Now()

	if v, ok := historyCache.Load(u.ID); ok {
		h := v.([]userHistory)
		h = append([]userHistory{{
			ProductID: pid,
			CreatedAt: now.Format("2006-01-02 15:04:05"),
		}}, h...)
		log.Printf("%#v\n", h)
		historyCache.Store(u.ID, h)
	}

	go db.Exec(
		"INSERT INTO histories (product_id, user_id, created_at) VALUES (?, ?, ?)",
		pid, u.ID, now)
}

// CreateComment : create comment to the product
func (u *User) CreateComment(pidStr string, content string) {
	now := time.Now()
	pid, _ := strconv.Atoi(pidStr)

	if v, ok := commentCache.Load(pid); ok {
		cs := v.([]Comment)
		cs = append([]Comment{{
			ProductID: pid,
			UserID:    u.ID,
			Content:   content,
			CreatedAt: now.Format("2006-01-02 15:04:05"),
		}}, cs...)
		commentCache.Store(pid, cs)
	}

	db.Exec(
		"INSERT INTO comments (product_id, user_id, content, created_at) VALUES (?, ?, ?, ?)",
		pidStr, u.ID, content, now)
}

func (u *User) UpdateLastLogin() {
	db.Exec("UPDATE users SET last_login = ? WHERE id = ?", time.Now(), u.ID)
}
